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
	remActivePeer chan *PeerConnection
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
	torrent.Info = file
	torrent.Swarm = []*Peer{}
	torrent.ActivePeers = []*PeerConnection{}
	torrent.TrackerList = []*Tracker{}
	torrent.newActivePeer = make(chan *PeerConnection)
	torrent.remActivePeer = make(chan *PeerConnection)
	torrent.newPeer = make(chan *Peer)
	torrent.remPeer = make(chan *Peer)
	torrent.peerInbox = make(chan peerMessage)
	torrent.remTracker = make(chan *Tracker)
	torrent.Download = 0
	torrent.Upload = 0
	torrent.Left = int64(len(file.Pieces) / 20)
	torrent.Picker = NewPiecePicker(&torrent, 16*1024)
	torrent.Sched = event.NewScheduler()

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
		mainAnnounce, err := NewTracker(*file.Announce, &torrent, ses)
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
				t.Swarm = nil
				t.ActivePeers = nil
				t.TrackerList = nil
				t.Sched = nil
				t.Picker = nil
				return
			}
		case p := <-t.newPeer:
			{
				t.Swarm = append(t.Swarm, p)
				fmt.Printf("PEER ADDED -> %v\n", p.Endpoint.String())
				if p.Conn == nil {
					go func() {
						retryIn, err := t.DialPeer(p)
						if err != nil {
							fmt.Printf("CONNECTION FAILED (retry in %v) -> %v\n", retryIn.Truncate(time.Second), err.Error())
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

				t.SchedulePeerKeepAlive(conn, time.Minute+time.Second*30)
				conn.SendBitfield(t.Picker.GetBitfield())
				if t.Picker.calculateInterested(conn) {
					conn.SendInterested(true)
				}
				fmt.Printf("CONNECTED -> %v\n", conn.Pid.String())
			}
		case conn := <-t.remActivePeer:
			{
				for i, val := range t.ActivePeers {
					if val == conn {
						t.ActivePeers = append(t.ActivePeers[:i], t.ActivePeers[i+1:]...)
						fmt.Printf(
							"DISCONNECTED -> %v BECAUSE: %v\n",
							conn.Pid.String(), context.Cause(conn.ctx).Error())
					}
				}
			}
		case p := <-t.remPeer:
			{
				for i, val := range t.Swarm {
					if val == p {
						fmt.Printf("PEER REMOVED BECAUSE: %v\n", context.Cause(p.ctx))
						t.Swarm = append(t.Swarm[:i], t.Swarm[i+1:]...)
					}
				}
			}
		case tracker := <-t.remTracker:
			{
				for i, val := range t.TrackerList {
					if val == tracker {
						fmt.Printf("[%v] -> TRACKER REMOVED\n", tracker.Announce.Host)
						t.TrackerList = append(t.TrackerList[:i], t.TrackerList[i+1:]...)
					}
				}
			}
		case mess := <-t.peerInbox:
			{
				t.handleIncomingMessage(mess)
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
		return retryIn, err
	}

	t.AddActiveConnection(peerConn)
	p.FailureCnt = 0
	return 0, nil
}

func (t *Torrent) Start() {
	go t.loop()
	for _, announce := range t.TrackerList {
		fmt.Printf("ANNOUNCING TO -> [%v]\n", announce.Announce.String())
		go func() {
			res, ok := announce.SendAnnounce(TrackerStarted)
			if !ok {
				return
			}
			t.ScheduleTrackerAnnounce(announce, time.Now().Add(time.Second*time.Duration(res.Interval)))
			for _, entry := range res.PeerList {
				peer := NewPeer(t, entry.IpPort)
				go t.AddPeer(peer)
			}
		}()
	}
}

func (t *Torrent) Stop() {
	t.cancel()
}

func (t *Torrent) AddActiveConnection(c *PeerConnection) {
	t.newActivePeer <- c
}

func (t *Torrent) RemoveActiveConnection(c *PeerConnection) {
	t.remActivePeer <- c
}

func (t *Torrent) AddPeer(p *Peer) {
	t.newPeer <- p
}

func (t *Torrent) RemovePeer(p *Peer) {
	t.remPeer <- p
}

func (t *Torrent) RemoveTracker(tracker *Tracker) {
	t.remTracker <- tracker
}

func (t *Torrent) ReceiveMessage(mess peerMessage) {
	t.peerInbox <- mess
}

func (t *Torrent) SchedulePeerConnectionRetry(p *Peer, retryIn time.Duration) {
	retryTask := event.Task{
		Fn: func() (time.Time, bool) {
			if p == nil || p.ctx.Err() != nil {
				return time.Now(), false
			}

			retryIn, err := t.DialPeer(p)
			if err != nil {
				if retryIn > time.Minute*30 {
					p.cancel(Peer_too_many_attempts)
					t.RemovePeer(p)
					fmt.Printf("CONNECTION FAILED (not retrying) -> %v\n", err.Error())
					return time.Now(), false
				}
				fmt.Printf("CONNECTION FAILED (retry in %v) -> %v\n", retryIn.Truncate(time.Second), err.Error())
				return time.Now().Add(retryIn), true
			}
			return time.Now(), false
		},
		RunAt: time.Now().Add(retryIn),
	}
	t.Sched.Schedule(retryTask)
}

func (t *Torrent) SchedulePeerKeepAlive(c *PeerConnection, freq time.Duration) {
	keepAliveTask := event.Task{
		Fn: func() (time.Time, bool) {
			if c == nil || c.ctx.Err() != nil {
				return time.Now(), false
			}

			go c.KeepAlive()
			fmt.Printf("KEEP ALIVE SENT -> %v\n", c.Pid.String())
			return time.Now().Add(freq), true
		},
		RunAt: time.Now().Add(freq),
	}
	t.Sched.Schedule(keepAliveTask)
}

func (t *Torrent) ScheduleTrackerAnnounce(announce *Tracker, interval time.Time) {
	announceTask := event.Task{
		Fn: func() (time.Time, bool) {
			if announce == nil || announce.ctx.Err != nil {
				return time.Now(), false
			}

			res, ok := announce.SendAnnounce(TrackerNone)
			if !ok {
				return time.Now(), false
			}
			for _, entry := range res.PeerList {
				peer := NewPeer(t, entry.IpPort)
				go t.AddPeer(peer)
			}
			return time.Now().Add(time.Second * time.Duration(res.Interval)), true
		},
		RunAt: interval,
	}
	t.Sched.Schedule(announceTask)
}

func (t *Torrent) handleIncomingMessage(mess peerMessage) {
	switch mess.Kind {
	case Choke:
		mess.Peer.AmChoked = true
	case Unchoke:
		mess.Peer.AmChoked = false
	case Interested:
		mess.Peer.AmInteresting = true
	case Uninterested:
		mess.Peer.AmInteresting = false
	case Have:
		idx := binary.LittleEndian.Uint32(mess.Payload)
		mess.Peer.Pieces.Set(idx, true)
		t.Picker.IncRef(idx)
	case Bitfield:
		ok := mess.Peer.Pieces.SetBitfield(mess.Payload)
		bf := utils.NewBitfield(t.Picker.pieceCount)
		bf.SetBitfield(mess.Payload)
		ok = t.Picker.IncRefBitfield(bf)
		if !ok {
			panic("unexpected error, bitfield size doesnt match")
		}
		mess.Peer.bitfieldSent = true
	}
}
