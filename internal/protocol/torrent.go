package protocol

import (
	"GoBit/internal/utils"
	"context"
	"fmt"
	"math"
	"net"
	"time"
)

type Torrent struct {
	ctx    context.Context
	cancel context.CancelFunc

	Info TorrentFile

	ActivePeers map[PeerID]ActivePeer
	Swarm       []*Peer
	TrackerList []*Tracker

	events chan Event
	tasks  chan Task

	Download int64
	Upload   int64
	Left     int64

	Ses    *Session
	Sched  *Scheduler
	Picker *PiecePicker
}

func NewTorrent(file TorrentFile, ses *Session) *Torrent {
	torrent := Torrent{}
	torrent.ctx, torrent.cancel = context.WithCancel(ses.ctx)
	torrent.Ses = ses
	torrent.Info = file
	torrent.Swarm = []*Peer{}
	torrent.ActivePeers = make(map[PeerID]ActivePeer)
	torrent.TrackerList = []*Tracker{}

	torrent.events = make(chan Event)
	torrent.tasks = make(chan Task)

	torrent.Download = 0
	torrent.Upload = 0
	torrent.Left = int64(len(file.Pieces) / 20)
	torrent.Picker = NewPiecePicker(&torrent)
	torrent.Sched = NewScheduler(torrent.tasks)

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
		case task := <-t.tasks:
			{
				t.handleScheduledTask(task)
			}
		case event := <-t.events:
			{
				t.handleEvent(event)
			}
		}
	}
}

func (t *Torrent) handleScheduledTask(task Task) {
	switch tsk := task.(type) {
	case PeerKeepAliveTsk:
		t.handlePeerKeepAliveTask(tsk)
	case PeerTryConnectionTsk:
		t.handlePeerTryConnectionTask(tsk)
	case TrackerNextAnnounceTsk:
		t.handleTrackerNextAnnounceTask(tsk)
	}
}

func (t *Torrent) handleEvent(event Event) {
	switch e := event.(type) {
	case PeerAddedEv:
		t.handlePeerAddedEvent(e)
	case PeerRemovedEv:
		t.handlePeerRemovedEvent(e)
	case PeerConnectedEv:
		t.handlePeerConnectedEvent(e)
	case PeerDisconnectedEv:
		t.handlePeerDisconnected(e)
	case PeerChokeEv:
		t.handlePeerChokeEvent(e)
	case PeerInterestedEv:
		t.handlePeerInterestedEvent(e)
	case PeerHaveEv:
		t.handlePeerHaveEvent(e)
	case PeerBitfieldEv:
		t.handlePeerBitfieldEvent(e)
	case PeerRequestEv:
		t.handlePeerRequestEvent(e)
	case PeerPieceEv:
		t.handlePeerPieceEvent(e)
	case PeerCancelEv:
		t.handlePeerCancelEvent(e)
	case TrackerAddedEv:
		t.handleTrackerAddedEvent(e)
	case TrackerRemovedEv:
		t.handleTrackerRemovedEvent(e)
	}
}

func (t *Torrent) handlePeerKeepAliveTask(tsk PeerKeepAliveTsk) {
	peer, ok := t.ActivePeers[tsk.Receiver]
	if !ok {
		return
	}
	peer.Conn.KeepAlive()

	fmt.Printf("KEEP ALIVE SENT -> %v\n", tsk.Receiver)

	keepAliveTsk := PeerKeepAliveTsk{
		Receiver: tsk.Receiver,
	}
	t.Sched.Schedule(keepAliveTsk, time.Now().Add(time.Minute+time.Second*30))
}

func (t *Torrent) handlePeerTryConnectionTask(tsk PeerTryConnectionTsk) {
	go func() {
		retryIn, err := t.DialPeer(tsk.Peer)
		if err != nil {
			if retryIn > time.Minute*30 {
				// fmt.Printf("CONNECTION FAILED (dropping peer) -> [%v] BECAUSE: %v\n", tsk.Peer.Endpoint, err)
				removeEv := PeerRemovedEv{
					Sender: tsk.Peer,
					Cause:  err,
				}
				t.SignalEvent(removeEv)
				return
			}
			// fmt.Printf("CONNECTION FAILED (retry in %v) -> [%v] BECAUSE: %v\n", retryIn, tsk.Peer.Endpoint, err)
			retry := PeerTryConnectionTsk{
				Peer: tsk.Peer,
			}
			t.Sched.Schedule(retry, time.Now().Add(retryIn))
		}
	}()
}

func (t *Torrent) handleTrackerNextAnnounceTask(tsk TrackerNextAnnounceTsk) {
	go func() {
		res, err := tsk.Tracker.SendAnnounce(tsk.Event, t, t.Ses.PeerID, t.Ses.Port)
		if tsk.Event == TrackerStopped {
			return
		}

		if err != nil {
			tsk.Tracker.FailureCnt++
			retryIn := (time.Minute) * time.Duration(math.Pow(2, float64(tsk.Tracker.FailureCnt)))
			if retryIn > time.Hour*2 {
				fmt.Printf("ANNOUNCE FAILED (dropping tracker) -> [%v] BECAUSE: %v\n", tsk.Tracker.Announce.String(), err)
				removeEv := TrackerRemovedEv{
					Sender: tsk.Tracker,
					Cause:  err,
				}
				t.SignalEvent(removeEv)
				return
			}
			fmt.Printf("ANNOUNCE FAILED (retry in %v) -> [%v] BECAUSE: %v\n", retryIn, tsk.Tracker.Announce.String(), err)
			retryTsk := TrackerNextAnnounceTsk{
				Tracker: tsk.Tracker,
				Event:   TrackerNone,
			}
			t.Sched.Schedule(retryTsk, time.Now().Add(retryIn))
			return
		}
		fmt.Printf("ANNOUNCED -> [%v] NEXT IN %v\n", tsk.Tracker.Announce.String(), time.Second*time.Duration(res.Interval))

		for _, entry := range res.PeerList {
			peer := NewPeer(entry.IpPort)
			addedEv := PeerAddedEv{
				Sender: peer,
			}
			t.SignalEvent(addedEv)
		}

		nextAnnounceTsk := TrackerNextAnnounceTsk{
			Tracker: tsk.Tracker,
			Event:   TrackerNone,
		}
		t.Sched.Schedule(nextAnnounceTsk, time.Now().Add(time.Second*time.Duration(res.Interval)))
	}()
}

func (t *Torrent) handlePeerAddedEvent(e PeerAddedEv) {
	t.Swarm = append(t.Swarm, e.Sender)
	fmt.Printf("PEER ADDED -> %v\n", e.Sender.Endpoint.String())
	if e.Sender.Conn == nil {
		tryConnectionTsk := PeerTryConnectionTsk{
			Peer: e.Sender,
		}
		t.Sched.Schedule(tryConnectionTsk, time.Now())
	}
}

func (t *Torrent) handlePeerRemovedEvent(e PeerRemovedEv) {
	for i, val := range t.Swarm {
		if val == e.Sender {
			t.Swarm = append(t.Swarm[:i], t.Swarm[i+1:]...)
			fmt.Printf("PEER REMOVED BECAUSE: %v\n", e.Cause)
		}
	}
}

func (t *Torrent) handlePeerConnectedEvent(e PeerConnectedEv) {
	state := ActivePeerState{
		LastTickTime:     time.Now(),
		Pieces:           utils.NewBitfield(t.Picker.pieceCount),
		IsChoked:         true,
		AmChoked:         true,
		IsInteresting:    false,
		AmInteresting:    false,
		TotalDownloaded:  0,
		TotalUploaded:    0,
		LastTickDownload: 0,
		LastTickUpload:   0,
	}
	peer := ActivePeer{e.Sender, state}
	t.ActivePeers[e.Sender.Pid] = peer

	e.Sender.attachTorrent(t)
	e.Sender.start()

	fmt.Printf("CONNECTED (attempts: %v) -> %v\n", e.Attempts, e.Sender.Pid.String())

	e.Sender.SendBitfield(t.Picker.GetBitfield())

	keepAliveTsk := PeerKeepAliveTsk{
		Receiver: e.Sender.Pid,
	}
	t.Sched.Schedule(keepAliveTsk, time.Now().Add(time.Minute+time.Second*30))
}

func (t *Torrent) handlePeerDisconnected(e PeerDisconnectedEv) {
	p, ok := t.ActivePeers[e.Sender]
	if ok {
		delete(t.ActivePeers, e.Sender)
		p.Conn.Peer.Conn = nil
		fmt.Printf("DISCONNECTED -> %v BECAUSE: %v\n", e.Sender.String(), e.Cause)
	}
}

func (t *Torrent) handlePeerChokeEvent(e PeerChokeEv) {
	val, ok := t.ActivePeers[e.Sender]
	fmt.Printf("UN/CHOKE -> %v\n", e.Sender)
	if !ok {
		panic(fmt.Errorf("received message from unknown peer -> %v", e.Sender))
	}
	val.State.AmChoked = e.Value
}

func (t *Torrent) handlePeerInterestedEvent(e PeerInterestedEv) {
	val, ok := t.ActivePeers[e.Sender]
	fmt.Printf("UN/INTERESTED -> %v\n", e.Sender)
	if !ok {
		panic(fmt.Errorf("received message from unknown peer -> %v", e.Sender))
	}
	val.State.AmInteresting = e.Value
}

func (t *Torrent) handlePeerHaveEvent(e PeerHaveEv) {
	val, ok := t.ActivePeers[e.Sender]
	fmt.Printf("HAVE -> %v\n", e.Sender)
	if !ok {
		panic(fmt.Errorf("received message from unknown peer -> %v", e.Sender))
	}
	val.State.Pieces.Set(uint32(e.Idx), true)
	t.Picker.IncRef(uint32(e.Idx))
}

func (t *Torrent) handlePeerBitfieldEvent(e PeerBitfieldEv) {
	val, ok := t.ActivePeers[e.Sender]
	fmt.Printf("BITFIELD -> %v\n", e.Sender)
	if !ok {
		panic(fmt.Errorf("received message from unknown peer -> %v", e.Sender))
	}
	val.State.Pieces = e.Bitfield
	ok = t.Picker.IncRefBitfield(e.Bitfield)
	if !ok {
		panic("unexpected error, bitfield size doesnt match")
	}

	if t.Picker.calculateInterested(val.State) {
		val.Conn.SendInterested(true)
	}
}

func (t *Torrent) handlePeerRequestEvent(e PeerRequestEv) {
	// TODO
}

func (t *Torrent) handlePeerPieceEvent(e PeerPieceEv) {
	// TODO
}

func (t *Torrent) handlePeerCancelEvent(e PeerCancelEv) {
	// TODO
}

func (t *Torrent) handleTrackerAddedEvent(e TrackerAddedEv) {
	t.TrackerList = append(t.TrackerList, e.Sender)

	announceTsk := TrackerNextAnnounceTsk{
		Tracker: e.Sender,
		Event:   TrackerStarted,
	}
	t.Sched.Schedule(announceTsk, time.Now())
	fmt.Printf("TRACKER ADDED -> [%v]\n", e.Sender.Announce.Host)
}

func (t *Torrent) handleTrackerRemovedEvent(e TrackerRemovedEv) bool {
	for i, val := range t.TrackerList {
		if val == e.Sender {
			announceTsk := TrackerNextAnnounceTsk{
				Tracker: e.Sender,
				Event:   TrackerStopped,
			}
			t.Sched.Schedule(announceTsk, time.Now())
			t.TrackerList = append(t.TrackerList[:i], t.TrackerList[i+1:]...)
			fmt.Printf("TRACKER REMOVED -> [%v] BECAUSE: %v\n", e.Sender.Announce.Host, e.Cause)
			return true
		}
	}
	return false
}

func (t *Torrent) DialPeer(p *Peer) (time.Duration, error) {
	conn, err := net.DialTimeout("tcp", p.Endpoint.String(), time.Second*3)
	peerConn := newPeerConnection(conn)
	peerConn.Peer = p

	if err == nil {
		err = peerConn.handshakePeer(t.Info.InfoHash, t.Ses.PeerID)
	}
	if err != nil {
		p.FailureCnt++
		retryIn := (time.Minute) * time.Duration(math.Pow(2, float64(p.FailureCnt)))
		return retryIn, err
	}

	connectedEv := PeerConnectedEv{
		Sender:   peerConn,
		Attempts: p.FailureCnt + 1,
	}

	t.SignalEvent(connectedEv)
	p.FailureCnt = 0
	return 0, nil
}

func (t *Torrent) Choke(p ActivePeer) {
	if p.State.IsChoked == true {
		return
	}
	p.Conn.SendChoked(true)
	p.State.IsChoked = true
}

func (t *Torrent) Unchoke(p ActivePeer) {
	if p.State.IsChoked == false {
		return
	}
	p.Conn.SendChoked(false)
	p.State.IsChoked = false
}

func (t *Torrent) SetInteresting(p ActivePeer) {
	if p.State.IsInteresting == true {
		return
	}
	p.Conn.SendInterested(true)
	p.State.IsInteresting = true
}

func (t *Torrent) SetUninteresting(p ActivePeer) {
	if p.State.IsInteresting == false {
		return
	}
	p.Conn.SendInterested(false)
	p.State.IsInteresting = false
}

func (t *Torrent) Start() {
	trackers := []*Tracker{}
	addedEv := TrackerAddedEv{
		Sender: nil,
	}

	if t.Info.AnnounceList != nil {
		for _, lst := range *t.Info.AnnounceList {
			for _, trackerUrl := range lst {
				announce, err := NewTracker(trackerUrl)
				if err == nil {
					trackers = append(trackers, announce)
				}
			}
		}
	} else {
		announce, err := NewTracker(*t.Info.Announce)
		if err == nil {
			trackers = append(trackers, announce)
		}
	}

	go t.loop()

	for _, tracker := range trackers {
		addedEv.Sender = tracker
		t.SignalEvent(addedEv)
	}
}

func (t *Torrent) Stop() {
	t.cancel()
}

func (t *Torrent) SignalEvent(e Event) {
	t.events <- e
}
