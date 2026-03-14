package protocol

import (
	"GoBit/internal/event"
	"GoBit/internal/utils"
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

type Torrent struct {
	ctx    context.Context
	cancel context.CancelFunc

	Ses         *Session
	PeerList    []*Peer
	TrackerList []*Tracker
	Info        TorrentFile

	newPeer   chan *Peer
	remPeer   chan *Peer
	peerInbox chan peerMessage

	remTracker   chan *Tracker
	trackerInbox chan TrackerResult

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
	torrent.PeerList = []*Peer{}
	torrent.TrackerList = []*Tracker{}
	torrent.newPeer = make(chan *Peer)
	torrent.remPeer = make(chan *Peer)
	torrent.peerInbox = make(chan peerMessage)
	torrent.remTracker = make(chan *Tracker)
	torrent.trackerInbox = make(chan TrackerResult)
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
				t.PeerList = append(t.PeerList, p)
				go t.DialPeerOrRetry(p)
			}
		case p := <-t.remPeer:
			{
				for i, val := range t.PeerList {
					if val == p {
						t.PeerList = append(t.PeerList[:i], t.PeerList[i+1:]...)
					}
				}
				t.Picker.DecRefBitfield(p.Pieces)
				if p.Pid == nil {
					fmt.Printf(
						"ANONYMOUS PEER REMOVED BECAUSE: %v\n",
						context.Cause(p.ctx))
				} else {
					fmt.Printf(
						"%v REMOVED BECAUSE: %v\n",
						p.Pid.String(), context.Cause(p.ctx))
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

func (t *Torrent) Start() {
	go t.loop()
	for _, announce := range t.TrackerList {
		//
		fmt.Printf("ANNOUNCING TO [%v]\n", announce.Announce.String())
		//
		go t.AnnounceAndSchedule(announce)
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

func (t *Torrent) dialPeer(p *Peer) (time.Time, bool) {
	err := p.TryConnect()

	if err != nil {
		delay := (time.Second * 30) * time.Duration(math.Pow(2, float64(p.failureCnt)))
		fmt.Printf("CONNECTION FAILED (should retry in %v) -> %v\n", delay.Truncate(time.Second), err.Error())
		return time.Now().Add(delay), true
	}

	fmt.Printf("CONNECTION SUCCESS -> %v\n", string(p.Pid.String()))
	p.SendBitfield(t.Picker.GetBitfield())
	if t.Picker.calculateInterested(p) {
		p.SendInterested(true)
	}

	return time.Now(), false
}

func (t *Torrent) DialPeerOrRetry(p *Peer) {
	retryAt, retry := t.dialPeer(p)
	if retry {
		retryTask := event.Task{
			Fn: func() (time.Time, bool) {
				return t.dialPeer(p)
			},
			RunAt: retryAt,
		}
		go t.Sched.Schedule(retryTask)
		return
	}

	keepAliveTask := event.Task{
		Fn: func() (time.Time, bool) {
			defer fmt.Println("KEEP ALIVE SENT")
			return p.Conn.keepAlive()
		},
		RunAt: time.Now().Add(time.Minute),
	}
	go t.Sched.Schedule(keepAliveTask)
}

func (t *Torrent) AnnounceAndSchedule(tracker *Tracker) {
	interval, ok := tracker.SendAnnounce(TrackerStarted)
	if !ok {
		t.RemoveTracker(tracker)
		return
	}

	announceTask := event.Task{
		Fn: func() (time.Time, bool) {
			return tracker.SendAnnounce(TrackerNone)
		},
		RunAt: interval,
	}

	go t.Sched.Schedule(announceTask)
}

func (t *Torrent) Stop() {
	t.cancel()
}

func (t *Torrent) RemovePeer(p *Peer) {
	t.remPeer <- p
}

func (t *Torrent) AddPeer(e PeerEndpoint) {
	p := NewPeer(t, e, t.Ses.PeerID)
	t.newPeer <- p
}

func (t *Torrent) ReceiveMessage(mess peerMessage) {
	t.peerInbox <- mess
}

func (t *Torrent) RemoveTracker(tracker *Tracker) {
	t.remTracker <- tracker
}

func (t *Torrent) ReceiveTracker(res TrackerResult) {
	t.trackerInbox <- res
}
