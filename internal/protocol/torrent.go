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
						conn, _, _, err := t.DialPeer(p)
						if err != nil {
							delay := (time.Minute) * time.Duration(math.Pow(2, float64(p.FailureCnt)))
							fmt.Printf("CONNECTION FAILED (retry in %v) -> %v\n", delay.Truncate(time.Second), err.Error())
							t.SchedulePeerConnectionRetry(p, delay)
							return
						}
						fmt.Printf("CONNECTION SUCCESS (attemps: 1) -> %v\n", string(conn.Pid.String()))
						p.FailureCnt = 0
						t.AddActivePeer(conn)
						t.SchedulePeerKeepAlive(conn)

						conn.SendBitfield(t.Picker.GetBitfield())
						if t.Picker.calculateInterested(conn) {
							conn.SendInterested(true)
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

func (t *Torrent) DialPeer(p *Peer) (*PeerConnection, int, time.Duration, error) {
	conn, err := net.DialTimeout("tcp", p.Endpoint.String(), time.Second*3)
	delay := (time.Minute) * time.Duration(math.Pow(2, float64(p.FailureCnt)))
	if err != nil {
		p.FailureCnt++
		return nil, 0, delay, err
	}

	peerConn := newPeerConnection(conn)
	err = peerConn.handshakePeer(t.Info.InfoHash, t.Ses.PeerID)

	if err != nil {
		p.FailureCnt++
		return nil, 0, delay, err
	}

	failureCnt := p.FailureCnt
	p.FailureCnt = 0
	return peerConn, failureCnt + 1, 0, nil
}

func (t *Torrent) SchedulePeerConnectionRetry(p *Peer, startingDelay time.Duration) {
	retryTask := event.Task{
		Fn: func() (time.Time, bool) {
			conn, attempts, retryIn, err := t.DialPeer(p)
			if err != nil {
				fmt.Printf("CONNECTION FAILED (retry in %v) -> %v\n", retryIn.Truncate(time.Second), err.Error())
				return time.Now().Add(retryIn), true
			}
			fmt.Printf("CONNECTION SUCCESS (attemps: %v) -> %v\n", attempts, string(conn.Pid.String()))

			t.AddActivePeer(conn)
			t.SchedulePeerKeepAlive(conn)

			conn.SendBitfield(t.Picker.GetBitfield())
			if t.Picker.calculateInterested(conn) {
				conn.SendInterested(true)
			}
			return time.Now(), false
		},
		RunAt: time.Now().Add(startingDelay),
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

func (t *Torrent) OnAnnounceResponse(entries []PeerEntry) {
	for _, entry := range entries {
		peer := NewPeer(entry.IpPort)
		t.AddPeer(peer)
	}
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
				mess.Peer.cancel(Peer_invalid_bitfield)
				return
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

			announceTask := event.Task{
				Fn: func() (time.Time, bool) {
					return announce.SendAnnounce(TrackerNone)
				},
				RunAt: interval,
			}

			go t.Sched.Schedule(announceTask)
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
