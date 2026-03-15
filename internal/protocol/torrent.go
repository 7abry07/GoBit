package protocol

import (
	"GoBit/internal/event"
	"GoBit/internal/utils"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"time"
)

type Torrent struct {
	ctx    context.Context
	cancel context.CancelFunc

	Ses         *Session
	Swarm       []*Peer
	ActivePeers []*PeerConnection
	TrackerList []*Tracker
	Info        TorrentFile

	newActivePeer chan *PeerConnection
	newPeer       chan *Peer
	remPeer       chan *Peer
	peerInbox     chan peerMessage

	remTracker chan *Tracker

	Download int64
	Upload   int64
	Left     int64

	Sched  *event.Scheduler
	Picker *PiecePicker
}

func NewTorrent(file TorrentFile, ses *Session) *Torrent {
	torrent := Torrent{}
	torrent.ctx, torrent.cancel = context.WithCancel(ses.ctx)
	torrent.Ses = ses
	torrent.Sched = event.NewScheduler()
	torrent.Info = file
	torrent.Swarm = []*Peer{}
	torrent.ActivePeers = []*PeerConnection{}
	torrent.TrackerList = []*Tracker{}
	torrent.newActivePeer = make(chan *PeerConnection)
	torrent.newPeer = make(chan *Peer)
	torrent.remPeer = make(chan *Peer)
	torrent.peerInbox = make(chan peerMessage)
	torrent.remTracker = make(chan *Tracker)
	torrent.Download = 0
	torrent.Upload = 0
	torrent.Left = int64(len(file.Pieces) / 20)
	torrent.Picker = NewPiecePicker(&torrent, 16*1024)

	if file.AnnounceList != nil {
		for _, lst := range *file.AnnounceList {
			for _, t := range lst {
				alreadyAdded := false
				for _, item := range torrent.TrackerList {
					if t == item.Announce {
						alreadyAdded = true
						break
					}
				}
				if alreadyAdded {
					continue
				}

				announce, err := NewTracker(t, &torrent, ses)
				if err == nil {
					torrent.TrackerList = append(torrent.TrackerList, announce)
				}
			}
		}
	} else {
		mainAnnounce, err := NewTracker(file.Announce, &torrent, ses)
		if err == nil {
			torrent.TrackerList = append(torrent.TrackerList, mainAnnounce)
		}
	}

	return &torrent
}

func (t *Torrent) loop() {
	for {
		select {
		case <-t.ctx.Done():
			{
				fmt.Println("TORRENT STOPPED")
				return
			}
		case p := <-t.newPeer:
			{
				t.Swarm = append(t.Swarm, p)
				if p.Conn == nil {
					go func() {
						retryIn, err := t.DialPeer(p)
						if err != nil {
							t.SchedulePeerConnectionRetry(p, retryIn)
							return
						}
					}()
				}
			}
		case conn := <-t.newActivePeer:
			{
				t.ActivePeers = append(t.ActivePeers, conn)
				conn.attachTorrent(t)
				conn.start()
			}
		case p := <-t.remPeer:
			{
				for i, val := range t.Swarm {
					if val == p {
						t.Swarm = append(t.Swarm[:i], t.Swarm[i+1:]...)
					}
				}
			}
		case mess := <-t.peerInbox:
			{
				go t.handleIncomingMessage(mess)
			}

		case tracker := <-t.remTracker:
			{
				for i, val := range t.TrackerList {
					if val == tracker {
						t.TrackerList = append(t.TrackerList[:i], t.TrackerList[i+1:]...)
					}
				}
				go tracker.SendAnnounce(TrackerStopped)
			}
		}
	}
}

func (t *Torrent) DialPeer(p *Peer) (time.Duration, error) {
	conn, err := net.DialTimeout("tcp", p.Endpoint.String(), time.Second*3)
	peerConn := newPeerConnection(conn)

	if err == nil {
		err = peerConn.handshakePeer(t.Info.InfoHash, t.Ses.PeerID)
	}
	if err != nil {
		p.FailureCnt++
		retryIn := (time.Minute) * time.Duration(math.Pow(2, float64(p.FailureCnt)))
		fmt.Printf("CONNECTION FAILED (retry in %v) -> %v\n", retryIn.Truncate(time.Second), err.Error())
		return retryIn, err
	}

	fmt.Printf("CONNECTION SUCCESS (attemps: %v) -> %v\n", p.FailureCnt+1, string(peerConn.Pid.String()))
	t.AddActivePeer(peerConn)
	t.SchedulePeerKeepAlive(peerConn)

	peerConn.SendBitfield(t.Picker.GetBitfield())
	if t.Picker.calculateInterested(peerConn) {
		peerConn.SendInterested(true)
	}
	p.FailureCnt = 0
	return 0, nil
}

func (t *Torrent) handleIncomingMessage(mess peerMessage) {
	switch mess.Kind {
	case Have:
		{
			idx := binary.LittleEndian.Uint32(mess.Payload)
			t.Picker.IncRef(idx)
		}
	case Bitfield:
		{
			bf := utils.NewBitfield(t.Picker.pieceCount)
			bf.SetBitfield(mess.Payload)
			ok := t.Picker.IncRefBitfield(bf)
			if !ok {
				panic("unexpected error, bitfield size doesnt match")
			}
		}
	}
}

func (t *Torrent) Start() {
	go t.loop()
	for _, announce := range t.TrackerList {
		fmt.Printf("ANNOUNCING TO -> [%v]\n", announce.Announce.String())
		go func() {
			interval, ok := announce.SendAnnounce(TrackerStarted)
			if !ok {
				t.RemoveTracker(announce)
				return
			}
			t.ScheduleTrackerAnnounce(announce, interval)
		}()
	}
}

func (t *Torrent) Stop() {
	t.cancel()
}

func (t *Torrent) RemovePeer(p *Peer) {
	t.remPeer <- p
}

func (t *Torrent) AddPeer(p *Peer) {
	t.newPeer <- p
}

func (t *Torrent) AddActivePeer(p *PeerConnection) {
	t.newActivePeer <- p
}

func (t *Torrent) ReceiveMessage(mess peerMessage) {
	t.peerInbox <- mess
}

func (t *Torrent) RemoveTracker(tracker *Tracker) {
	t.remTracker <- tracker
}

func (t *Torrent) SchedulePeerConnectionRetry(p *Peer, retryIn time.Duration) {
	retryTask := event.Task{
		Fn: func() (time.Time, bool) {
			retryIn, err := t.DialPeer(p)
			if err != nil {
				return time.Now().Add(retryIn), true
			}
			return time.Now(), false
		},
		RunAt: time.Now().Add(retryIn),
	}
	go t.Sched.Schedule(retryTask)
}

func (t *Torrent) SchedulePeerKeepAlive(c *PeerConnection) {
	keepAliveTask := event.Task{
		Fn: func() (time.Time, bool) {
			defer fmt.Printf("KEEP ALIVE SENT -> %v\n", c.Pid.String())
			if c == nil {
				return time.Now(), false
			}
			go c.KeepAlive()
			return time.Now().Add(c.keepAliveFreq), true
		},
		RunAt: time.Now().Add(c.keepAliveFreq),
	}
	go t.Sched.Schedule(keepAliveTask)
}

func (t *Torrent) ScheduleTrackerAnnounce(announce *Tracker, interval time.Time) {
	announceTask := event.Task{
		Fn: func() (time.Time, bool) {
			return announce.SendAnnounce(TrackerNone)
		},
		RunAt: interval,
	}
	go t.Sched.Schedule(announceTask)
}
