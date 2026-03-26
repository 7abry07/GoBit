package protocol

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"net"
	"slices"
	"time"

	"github.com/bits-and-blooms/bitset"
)

type Torrent struct {
	ctx    context.Context
	cancel context.CancelFunc

	Info TorrentFile

	ActivePeers map[PeerID]ActivePeer
	Swarm       []*Peer
	TrackerList []*Tracker

	optimisticUnchoke *PeerID

	events chan Event
	tasks  chan Task

	bitfield   bitset.BitSet
	Downloaded int64
	Uploaded   int64
	Left       int64

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

	torrent.optimisticUnchoke = nil

	torrent.events = make(chan Event, 1024)
	torrent.tasks = make(chan Task, 1024)

	torrent.bitfield = *bitset.New(uint(len(file.Pieces) / 20))
	torrent.Downloaded = 0
	torrent.Uploaded = 0
	torrent.Left = int64(len(file.Pieces) / 20)
	torrent.Picker = NewPiecePicker(&torrent)
	torrent.Sched = NewScheduler(&torrent)

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
	case PeerCalculateStatsTsk:
		t.handlePeerCalculateStatsTask(tsk)
	case TrackerNextAnnounceTsk:
		t.handleTrackerNextAnnounceTask(tsk)
	case ChokerTsk:
		t.handleChokerTask(tsk)
	case OptimisticUnchokeTsk:
		t.handleOptimisticUnchokeTask(tsk)
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
		t.handlePeerDisconnectedEvent(e)
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
	case PieceCompletedEv:
		t.handlePieceCompletedEvent(e)
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
	go peer.Conn.KeepAlive()
	// fmt.Printf("KEEP ALIVE SENT -> %v\n", tsk.Receiver)
	t.Sched.Schedule(
		PeerKeepAliveTsk{tsk.Receiver},
		time.Now().Add(time.Minute+time.Second*30))
}

func (t *Torrent) handlePeerTryConnectionTask(tsk PeerTryConnectionTsk) {
	go func() {
		retryIn, err := t.DialPeer(tsk.Peer)
		if err != nil {
			if retryIn > time.Minute*30 {
				// fmt.Printf("CONNECTION FAILED (dropping peer) -> [%v] BECAUSE: %v\n", tsk.Peer.Endpoint, err)
				t.SignalEvent(PeerRemovedEv{tsk.Peer, err})
				return
			}
			// fmt.Printf("CONNECTION FAILED (retry in %v) -> [%v] BECAUSE: %v\n", retryIn, tsk.Peer.Endpoint, err)
			t.Sched.Schedule(
				PeerTryConnectionTsk{tsk.Peer},
				time.Now().Add(retryIn))
		}
	}()
}

func (t *Torrent) handlePeerCalculateStatsTask(tsk PeerCalculateStatsTsk) {
	peer, ok := t.ActivePeers[tsk.Receiver]
	if !ok {
		return
	}
	now := time.Now()
	dt := now.Sub(peer.State.LastTickTime).Seconds()
	deltaDownload := peer.State.TotalDownloaded - peer.State.LastTickDownload
	deltaUpload := peer.State.TotalUploaded - peer.State.LastTickUpload
	instantDrate := float64(deltaDownload) / dt
	instantUrate := float64(deltaUpload) / dt

	const alpha = 0.3
	peer.State.DownloadRate = alpha*instantDrate + (1-alpha)*peer.State.DownloadRate
	peer.State.UploadRate = alpha*instantUrate + (1-alpha)*peer.State.UploadRate

	peer.State.LastTickDownload = peer.State.TotalDownloaded
	peer.State.LastTickUpload = peer.State.TotalUploaded
	peer.State.LastTickTime = now

	t.Sched.Schedule(
		PeerCalculateStatsTsk{tsk.Receiver},
		time.Now().Add(time.Second))
}

func (t *Torrent) handleTrackerNextAnnounceTask(tsk TrackerNextAnnounceTsk) {
	ih := t.Info.InfoHash
	d := t.Downloaded
	u := t.Uploaded
	l := t.Left

	go func() {
		res, err := tsk.Tracker.SendAnnounce(
			ih,
			d,
			u,
			l,
			tsk.Event,
			t.Ses.PeerID,
			t.Ses.Port)

		if tsk.Event == TrackerStopped {
			return
		}

		if err != nil {
			tsk.Tracker.FailureCnt++
			retryIn := (time.Minute) * time.Duration(math.Pow(2, float64(tsk.Tracker.FailureCnt)))
			if retryIn > time.Hour*2 {
				// fmt.Printf("ANNOUNCE FAILED (dropping tracker) -> [%v] BECAUSE: %v\n", tsk.Tracker.Announce.String(), err)
				t.SignalEvent(TrackerRemovedEv{tsk.Tracker, err})
				return
			}
			// fmt.Printf("ANNOUNCE FAILED (retry in %v) -> [%v] BECAUSE: %v\n", retryIn, tsk.Tracker.Announce.String(), err)
			t.Sched.Schedule(
				TrackerNextAnnounceTsk{tsk.Tracker, TrackerNone},
				time.Now().Add(retryIn))
			return
		}
		fmt.Printf("ANNOUNCED -> [%v] NEXT IN %v\n", tsk.Tracker.Announce.String(), time.Second*time.Duration(res.Interval))

		for _, entry := range res.PeerList {
			peer := NewPeer(entry.IpPort)
			t.SignalEvent(PeerAddedEv{peer})
		}

		t.Sched.Schedule(
			TrackerNextAnnounceTsk{tsk.Tracker, TrackerNone},
			time.Now().Add(time.Second*time.Duration(res.Interval)))
	}()
}

func (t *Torrent) handleChokerTask(tsk ChokerTsk) {
	activePeers := []ActivePeer{}
	for _, val := range t.ActivePeers {
		activePeers = append(activePeers, val)
	}

	peersToUnchoke := UnchokeSort(activePeers)
	for i, peer := range activePeers {
		if i < peersToUnchoke {
			t.Unchoke(peer)
			if peer.State.IsOptimistic {
				optimistic := t.ActivePeers[*t.optimisticUnchoke]
				optimistic.State.IsOptimistic = false
				t.optimisticUnchoke = nil
				// fmt.Printf("UNCHOKED -> %v (previously optimistic)\n", peer.Conn.Pid)
			} else {
				// fmt.Printf("UNCHOKED -> %v (download: %.1fkb | upload: %.1fkb)\n", peer.Conn.Pid, peer.State.DownloadRate/1024, peer.State.UploadRate/1024)
			}
		} else {
			if !peer.State.IsOptimistic {
				t.Choke(peer)
			}
		}
	}

	t.Sched.Schedule(ChokerTsk{}, time.Now().Add(time.Second*10))
}

func (t *Torrent) handleOptimisticUnchokeTask(tsk OptimisticUnchokeTsk) {
	// TODO
	chokedPeers := []ActivePeer{}

	for _, peer := range t.ActivePeers {
		if peer.State.IsChoked {
			chokedPeers = append(chokedPeers, peer)
		}
	}

	if len(chokedPeers) != 0 {
		optimistic := chokedPeers[rand.IntN(len(chokedPeers))]
		if t.optimisticUnchoke != nil {
			previousOptimistic := t.ActivePeers[*t.optimisticUnchoke]
			previousOptimistic.State.IsOptimistic = false
		}
		t.optimisticUnchoke = &optimistic.Conn.Pid
		optimistic.State.IsOptimistic = true
		t.Unchoke(optimistic)
		// fmt.Printf("OPTIMISTICALLY UNCHOKED -> %v\n", optimistic.Conn.Pid)
	}

	t.Sched.Schedule(
		OptimisticUnchokeTsk{},
		time.Now().Add(time.Second*30),
	)
}

// ------------------------------------------------------------------
// ------------------------------------------------------------------

func (t *Torrent) handlePeerAddedEvent(e PeerAddedEv) {
	t.Swarm = append(t.Swarm, e.Sender)
	// fmt.Printf("PEER ADDED -> %v\n", e.Sender.Endpoint.String())
	if e.Sender.Conn == nil {
		t.Sched.Schedule(
			PeerTryConnectionTsk{e.Sender},
			time.Now())
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
		Pieces:           bitset.New(t.Picker.pieceCount),
		IsChoked:         true,
		AmChoked:         true,
		IsInteresting:    false,
		AmInteresting:    false,
		TotalDownloaded:  0,
		TotalUploaded:    0,
		LastTickDownload: 0,
		LastTickUpload:   0,
	}
	peer := ActivePeer{e.Sender, &state}
	t.ActivePeers[e.Sender.Pid] = peer

	e.Sender.attachTorrent(t)
	e.Sender.start()

	fmt.Printf("CONNECTED (attempts: %v) -> %v\n", e.Attempts, e.Sender.Pid.String())

	e.Sender.SendBitfield(t.bitfield)

	t.Sched.Schedule(
		PeerKeepAliveTsk{e.Sender.Pid},
		time.Now().Add(time.Minute+time.Second*30))

	t.Sched.Schedule(
		PeerCalculateStatsTsk{e.Sender.Pid},
		time.Now().Add(time.Second))
}

func (t *Torrent) handlePeerDisconnectedEvent(e PeerDisconnectedEv) {
	peer, ok := t.ActivePeers[e.Sender]
	if ok {
		peer.Conn.Peer.PrevTotalDownloaded = peer.State.TotalDownloaded
		peer.Conn.Peer.PrevTotalUploaded = peer.State.TotalUploaded
		peer.Conn.Peer.Conn = nil
		t.Picker.DecRefBitfield(peer.State.Pieces)
		for _, req := range peer.State.PendingRequests {
			t.Picker.removeBlock(req.Idx, req.Begin/t.Info.BlockLength)
			// fmt.Printf("REQUEST REMOVED -> (%v:%v)\n", req.Idx, req.Begin/t.Info.BlockLength)
		}
		fmt.Printf("DISCONNECTED -> %v BECAUSE: %v\n", e.Sender.String(), e.Cause)
	}
}

func (t *Torrent) handlePeerChokeEvent(e PeerChokeEv) {
	// fmt.Printf("UN/CHOKE -> %v\n", e.Sender)
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		panic("choke")
	}
	peer.State.AmChoked = e.Value

	if !e.Value {
		for len(peer.State.PendingRequests) < 10 {
			pieceIdx := t.Picker.PickPiece(*peer.State)
			blockIdx := t.Picker.getLowestFreeBlock(pieceIdx)
			t.Picker.setBlockState(pieceIdx, blockIdx, BLOCK_REQUESTED)
			t.Picker.setPieceState(pieceIdx, PIECE_DOWNLOADING)

			request := PeerRequest{
				Idx:    pieceIdx,
				Begin:  blockIdx * t.Info.BlockLength,
				Length: t.Info.BlockLength,
			}

			peer.State.PendingRequests = append(peer.State.PendingRequests, request)
			peer.Conn.SendRequest(request)
		}
	} else {
		for _, req := range peer.State.PendingRequests {
			t.Picker.removeBlock(req.Idx, req.Begin/t.Info.BlockLength)
			// fmt.Printf("REQUEST REMOVED -> (%v:%v)\n", req.Idx, req.Begin/t.Info.BlockLength)
		}
		peer.State.PendingRequests = nil
	}
}

func (t *Torrent) handlePeerInterestedEvent(e PeerInterestedEv) {
	// fmt.Printf("UN/INTERESTED -> %v\n", e.Sender)
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		panic("interested")
	}
	peer.State.AmInteresting = e.Value
}

func (t *Torrent) handlePeerHaveEvent(e PeerHaveEv) {
	// fmt.Printf("HAVE -> %v\n", e.Sender)
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		panic("have")
	}
	peer.State.Pieces.Set(uint(e.Idx))
	t.Picker.IncRef(e.Idx)
}

func (t *Torrent) handlePeerBitfieldEvent(e PeerBitfieldEv) {
	// fmt.Printf("BITFIELD -> %v\n", e.Sender)
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		panic("bitfield")
	}
	peer.State.Pieces = e.Bitfield
	t.Picker.IncRefBitfield(e.Bitfield)

	if t.Picker.calculateInterested(*peer.State) {
		t.SetInteresting(peer)
	}
}

func (t *Torrent) handlePeerRequestEvent(e PeerRequestEv) {
	fmt.Printf("REQUEST -> [%v]\n", e.Sender)
}

func (t *Torrent) handlePeerPieceEvent(e PeerPieceEv) {
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		panic("piece")
	}
	// fmt.Printf("PIECE (%v:%v) -> %v\n", e.Idx, e.Begin/t.Info.BlockLength, e.Sender)
	for i, req := range peer.State.PendingRequests {
		if req.Begin == e.Begin {
			peer.State.TotalUploaded += len(e.Block)
			peer.State.PendingRequests = append(peer.State.PendingRequests[:i], peer.State.PendingRequests[i+1:]...)
			t.Picker.setBlockState(e.Idx, e.Begin/t.Info.BlockLength, BLOCK_RECEIVED)
			t.Picker.setBlockData(e.Idx, e.Begin/t.Info.BlockLength, e.Block)

			if t.Picker.isPieceComplete(e.Idx) {
				fmt.Printf("PIECE COMPLETED -> [%v]\n", e.Idx)
				t.Picker.setPieceState(e.Idx, PIECE_COMPLETE)
				t.SignalEvent(PieceCompletedEv{e.Idx})
			}

			newPieceIdx := t.Picker.PickPiece(*peer.State)
			newBlockIdx := t.Picker.getLowestFreeBlock(newPieceIdx)
			t.Picker.setBlockState(newPieceIdx, newBlockIdx, BLOCK_REQUESTED)
			t.Picker.setPieceState(newPieceIdx, PIECE_DOWNLOADING)

			newReq := PeerRequest{
				Idx:    newPieceIdx,
				Begin:  newBlockIdx * t.Info.BlockLength,
				Length: t.Info.BlockLength,
			}

			peer.State.PendingRequests = append(peer.State.PendingRequests, newReq)
			peer.Conn.SendRequest(newReq)

			return
		}
	}
}

func (t *Torrent) handlePeerCancelEvent(e PeerCancelEv) {
	// TODO
}

func (t *Torrent) handlePieceCompletedEvent(e PieceCompletedEv) {
	pieceHash := t.Picker.getPieceHash(e.Idx)
	actualPiecehash := t.Info.Pieces[e.Idx*20 : (e.Idx*20)+20]
	if slices.Compare(pieceHash, actualPiecehash) != 0 {
		t.Picker.resetPiece(e.Idx)
		fmt.Printf("HASH CHECK FAILED -> [%v]\n", e.Idx)
		return
	}

	t.Picker.setPieceState(e.Idx, PIECE_HAVE)
	t.bitfield.Set(uint(e.Idx))
	t.Downloaded++
	t.Left--
	go func() {
		for _, peer := range t.ActivePeers {
			go peer.Conn.SendHave(e.Idx)
		}
	}()
}

func (t *Torrent) handleTrackerAddedEvent(e TrackerAddedEv) {
	t.TrackerList = append(t.TrackerList, e.Sender)

	t.Sched.Schedule(
		TrackerNextAnnounceTsk{e.Sender, TrackerStarted},
		time.Now())

	fmt.Printf("TRACKER ADDED -> [%v]\n", e.Sender.Announce.Host)
}

func (t *Torrent) handleTrackerRemovedEvent(e TrackerRemovedEv) bool {
	for i, val := range t.TrackerList {
		if val == e.Sender {
			t.Sched.Schedule(
				TrackerNextAnnounceTsk{e.Sender, TrackerStopped},
				time.Now())

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

	t.SignalEvent(PeerConnectedEv{peerConn, p.FailureCnt + 1})

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
		t.SignalEvent(
			TrackerAddedEv{
				Sender: tracker,
			})
	}
	now := time.Now()
	t.Sched.Schedule(ChokerTsk{}, now.Add(time.Second*10))
	t.Sched.Schedule(OptimisticUnchokeTsk{}, now.Add(time.Second*30))
}

func (t *Torrent) Stop() {
	t.cancel()
}

func (t *Torrent) SignalEvent(e Event) {
	t.events <- e
}

func (t *Torrent) SignalTask(tsk Task) {
	t.tasks <- tsk
}
