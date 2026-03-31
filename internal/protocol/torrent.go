package protocol

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"net"
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

	Ses     *Session
	Sched   *Scheduler
	Picker  *PiecePicker
	DiskMan *DiskManager
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

	pieceCount := len(torrent.Info.Pieces) / 20

	torrent.bitfield = *bitset.New(uint(pieceCount))
	torrent.Left = int64(pieceCount)
	torrent.Downloaded = 0
	torrent.Uploaded = 0

	totalSize := uint64(0)
	if torrent.Info.FileMode() == multi {
		for _, file := range torrent.Info.Files {
			totalSize += file.Length
		}
	} else {
		totalSize = *torrent.Info.Length
	}

	torrent.DiskMan = NewDiskManager(&torrent, totalSize, uint32(pieceCount), torrent.Info.PieceSize, torrent.Info.BlockSize)
	torrent.Picker = NewPiecePicker(&torrent, totalSize, uint32(pieceCount), torrent.Info.PieceSize, torrent.Info.BlockSize)
	torrent.Sched = NewScheduler(&torrent)

	torrent.DiskMan.RootName = torrent.Info.Name

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
				t.Picker = nil

				for _, tracker := range t.TrackerList {
					tracker.SendAnnounce(
						t.Info.InfoHash,
						t.Downloaded,
						t.Uploaded,
						t.Left,
						TRACKER_STOPPED,
						t.Ses.PeerID,
						t.Ses.Port)
				}

				t.TrackerList = nil
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
	case TrackerTryAnnounceTsk:
		t.handleTrackerTryAnnounceTask(tsk)
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
	case PeerConnectionFailedEv:
		t.handlePeerConnectionFailedEvent(e)
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
	case TrackerAnnounceSuccessfulEv:
		t.handleTrackerAnnounceSuccesfulEvent(e)
	case TrackerAnnounceFailedEv:
		t.handleTrackerAnnounceFailedEvent(e)
	case DiskWriteSuccessfulEv:
		t.handleDiskWriteSuccessfulEvent(e)
	case DiskWriteFailedEv:
		t.handleDiskWriteFailedEvent(e)
	case DiskHashSuccessfulEv:
		t.handleDiskHashSuccessfulEvent(e)
	case DiskHashFailedEv:
		t.handleDiskHashFailedEvent(e)
	}
}

func (t *Torrent) handlePeerKeepAliveTask(tsk PeerKeepAliveTsk) {
	peer, ok := t.ActivePeers[tsk.Receiver]
	if !ok {
		return
	}
	peer.Conn.KeepAlive()
	// fmt.Printf("KEEP ALIVE SENT -> %v\n", tsk.Receiver)
	t.Sched.Schedule(
		PeerKeepAliveTsk{tsk.Receiver},
		time.Now().Add(time.Minute))
}

func (t *Torrent) handlePeerTryConnectionTask(tsk PeerTryConnectionTsk) {
	go func() {
		conn, err := net.DialTimeout("tcp", tsk.Peer.Endpoint.String(), time.Second*3)
		peerConn := newPeerConnection(conn)
		peerConn.Peer = tsk.Peer

		if err == nil {
			err = peerConn.handshakePeer(t.Info.InfoHash, t.Ses.PeerID)
		}

		if err != nil {
			t.SignalEvent(PeerConnectionFailedEv{tsk.Peer, err})
		} else {
			t.SignalEvent(PeerConnectedEv{peerConn, tsk.Peer.FailureCnt + 1})
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

func (t *Torrent) handleTrackerTryAnnounceTask(tsk TrackerTryAnnounceTsk) {
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

		if err != nil {
			t.SignalEvent(TrackerAnnounceFailedEv{tsk.Tracker, err})
		} else {
			t.SignalEvent(TrackerAnnounceSuccessfulEv{tsk.Tracker, res})
		}
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
			if t.optimisticUnchoke != nil {
				previousOptimistic := t.ActivePeers[*t.optimisticUnchoke]
				previousOptimistic.State.IsOptimistic = false
			}
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

//
// PEER EVENT HANDLERS
//

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
		Pieces:           bitset.New(uint(t.Picker.pieceCount)),
		IsChoked:         true,
		AmChoked:         true,
		IsInteresting:    false,
		AmInteresting:    false,
		TotalDownloaded:  0,
		TotalUploaded:    0,
		LastTickDownload: 0,
		LastTickUpload:   0,
	}

	e.Sender.Peer.FailureCnt = 0
	peer := ActivePeer{e.Sender, &state}
	t.ActivePeers[e.Sender.Pid] = peer

	e.Sender.attachTorrent(t)
	e.Sender.start()

	fmt.Printf("CONNECTED (attempts: %v) -> %v\n", e.Attempts, e.Sender.Pid.String())

	e.Sender.SendBitfield(t.bitfield)

	t.Sched.Schedule(
		PeerKeepAliveTsk{e.Sender.Pid},
		time.Now().Add(time.Second*20))
	time.Now().Add(time.Minute)

	t.Sched.Schedule(
		PeerCalculateStatsTsk{e.Sender.Pid},
		time.Now().Add(time.Second))
}

func (t *Torrent) handlePeerDisconnectedEvent(e PeerDisconnectedEv) {
	peer, ok := t.ActivePeers[e.Sender]
	if ok {
		delete(t.ActivePeers, peer.Conn.Pid)
		peer.Conn.Peer.PrevTotalDownloaded = peer.State.TotalDownloaded
		peer.Conn.Peer.PrevTotalUploaded = peer.State.TotalUploaded
		peer.Conn.Peer.Conn = nil
		t.Picker.DecRefBitfield(peer.State.Pieces)
		for _, req := range peer.State.PendingRequests {
			// fmt.Printf("[%v] REMOVING REQUEST -> (%v:%v) \n", peer.Conn.Pid, req.Idx, req.Begin/t.Info.BlockSize)
			t.Picker.removeBlock(req.Idx, req.Begin/t.Info.BlockSize)
		}

		if peer.State.IsOptimistic {
			t.optimisticUnchoke = nil
		}

		peer.State.PendingRequests = nil
		fmt.Printf("DISCONNECTED -> %v BECAUSE: %v\n", e.Sender.String(), e.Cause)
	}
}

func (t *Torrent) handlePeerConnectionFailedEvent(e PeerConnectionFailedEv) {
	e.Sender.FailureCnt++
	retryIn := (time.Minute) * time.Duration(math.Pow(2, float64(e.Sender.FailureCnt)))
	if retryIn > time.Minute*30 {
		// fmt.Printf("CONNECTION FAILED (dropping peer) -> [%v] BECAUSE: %v\n", e.Sender.Endpoint, e.Err)
		t.SignalEvent(PeerRemovedEv{e.Sender, e.Err})
	} else {
		// fmt.Printf("CONNECTION FAILED (retry in %v) -> [%v] BECAUSE: %v\n", retryIn, e.Sender.Endpoint, e.Err)
		t.Sched.Schedule(
			PeerTryConnectionTsk{e.Sender},
			time.Now().Add(retryIn))
	}

}

func (t *Torrent) handlePeerChokeEvent(e PeerChokeEv) {
	// fmt.Printf("UN/CHOKE -> %v\n", e.Sender)
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		return
	}
	peer.State.AmChoked = e.Value

	if !e.Value {
		if !peer.State.IsInteresting {
			return
		}
		for len(peer.State.PendingRequests) < 10 {
			pieceIdx, ok := t.Picker.PickPiece(*peer.State)
			if !ok {
				return
			}

			blockIdx, ok := t.Picker.getLowestFreeBlock(pieceIdx)
			if !ok {
				return
			}

			t.Picker.setBlockState(pieceIdx, blockIdx, BLOCK_REQUESTED)
			t.Picker.setPieceState(pieceIdx, PIECE_DOWNLOADING)

			request := PeerRequest{
				Idx:    pieceIdx,
				Begin:  blockIdx * t.Info.BlockSize,
				Length: t.Info.BlockSize,
			}

			peer.State.PendingRequests = append(peer.State.PendingRequests, request)
			peer.Conn.SendRequest(request)
		}
	} else {
		for _, req := range peer.State.PendingRequests {
			// fmt.Printf("[%v] REMOVING REQUEST -> (%v:%v) \n", peer.Conn.Pid, req.Idx, req.Begin/t.Info.BlockSize)
			t.Picker.removeBlock(req.Idx, req.Begin/t.Info.BlockSize)
		}
		peer.State.PendingRequests = nil
	}
}

func (t *Torrent) handlePeerInterestedEvent(e PeerInterestedEv) {
	// fmt.Printf("UN/INTERESTED -> %v\n", e.Sender)
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		return
	}
	peer.State.AmInteresting = e.Value
}

func (t *Torrent) handlePeerHaveEvent(e PeerHaveEv) {
	// fmt.Printf("HAVE -> %v\n", e.Sender)
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		return
	}
	peer.State.Pieces.Set(uint(e.Idx))
	t.Picker.IncRef(e.Idx)

	if !t.bitfield.Test(uint(e.Idx)) {
		t.SetInteresting(peer)
	}
}

func (t *Torrent) handlePeerBitfieldEvent(e PeerBitfieldEv) {
	// fmt.Printf("BITFIELD -> %v\n", e.Sender)
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		return
	}
	peer.State.Pieces = e.Bitfield
	t.Picker.IncRefBitfield(e.Bitfield)

	if t.Picker.calculateInterested(*peer.State) {
		t.SetInteresting(peer)
	}
}

func (t *Torrent) handlePeerRequestEvent(e PeerRequestEv) {
	fmt.Printf("REQUEST -> [%v]\n", e.Sender)
	t.DiskMan.EnqueueJob(DiskReadJob{
		e.Sender, e.Idx, e.Begin / t.Info.BlockSize, e.Length,
	})
}

func (t *Torrent) handlePeerPieceEvent(e PeerPieceEv) {
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		return
	}
	// fmt.Printf("PIECE (%v:%v) -> %v\n", e.Idx, e.Begin/t.Info.BlockSize, e.Sender)
	for i, req := range peer.State.PendingRequests {
		if req.Begin == e.Begin {
			peer.State.TotalUploaded += len(e.Block)
			peer.State.PendingRequests = append(peer.State.PendingRequests[:i], peer.State.PendingRequests[i+1:]...)
			t.Picker.setBlockState(e.Idx, e.Begin/t.Info.BlockSize, BLOCK_RECEIVED)
			t.DiskMan.EnqueueJob(DiskWriteJob{
				e.Idx, e.Begin / t.Info.BlockSize, e.Block,
			})

			newPieceIdx, ok := t.Picker.PickPiece(*peer.State)
			if !ok {
				return
			}

			newBlockIdx, ok := t.Picker.getLowestFreeBlock(newPieceIdx)
			if !ok {
				return
			}

			t.Picker.setBlockState(newPieceIdx, newBlockIdx, BLOCK_REQUESTED)
			t.Picker.setPieceState(newPieceIdx, PIECE_DOWNLOADING)

			newReq := PeerRequest{
				Idx:    newPieceIdx,
				Begin:  newBlockIdx * t.Info.BlockSize,
				Length: t.Info.BlockSize,
			}

			peer.State.PendingRequests = append(peer.State.PendingRequests, newReq)
			peer.Conn.SendRequest(newReq)
		}
	}
}

func (t *Torrent) handlePeerCancelEvent(e PeerCancelEv) {
	// TODO
}

func (t *Torrent) handlePieceCompletedEvent(e PieceCompletedEv) {
	fmt.Printf("PIECE COMPLETED -> [%v]\n", e.Idx)
	t.Picker.setPieceState(e.Idx, PIECE_COMPLETE)
	t.Picker.deletePieceBlockData(e.Idx)
	t.DiskMan.EnqueueJob(DiskHashJob{
		e.Idx,
	})
}

//
// TRACKER EVENT HANDLERS
//

func (t *Torrent) handleTrackerAddedEvent(e TrackerAddedEv) {
	t.TrackerList = append(t.TrackerList, e.Sender)

	t.Sched.Schedule(
		TrackerTryAnnounceTsk{e.Sender, TRACKER_STARTED},
		time.Now())

	fmt.Printf("TRACKER ADDED -> [%v]\n", e.Sender.Announce.Host)
}

func (t *Torrent) handleTrackerRemovedEvent(e TrackerRemovedEv) {
	for i, val := range t.TrackerList {
		if val == e.Sender {
			t.TrackerList = append(t.TrackerList[:i], t.TrackerList[i+1:]...)
			fmt.Printf("TRACKER REMOVED -> [%v] BECAUSE: %v\n", e.Sender.Announce.Host, e.Cause)
		}
	}
}

func (t *Torrent) handleTrackerAnnounceSuccesfulEvent(e TrackerAnnounceSuccessfulEv) {
	fmt.Printf("ANNOUNCED -> [%v] NEXT IN %v\n", e.Sender.Announce.String(), time.Second*time.Duration(e.Response.Interval))

	for _, entry := range e.Response.PeerList {
		peer := NewPeer(entry.IpPort)
		t.SignalEvent(PeerAddedEv{peer})
	}

	t.Sched.Schedule(
		TrackerTryAnnounceTsk{e.Sender, TRACKER_NONE},
		time.Now().Add(time.Second*time.Duration(e.Response.Interval)))
}

func (t *Torrent) handleTrackerAnnounceFailedEvent(e TrackerAnnounceFailedEv) {
	e.Sender.FailureCnt++
	retryIn := (time.Minute) * time.Duration(math.Pow(2, float64(e.Sender.FailureCnt)))
	if retryIn > time.Hour*2 {
		fmt.Printf("ANNOUNCE FAILED (dropping tracker) -> [%v] BECAUSE: %v\n", e.Sender.Announce.String(), e.Err)
		t.SignalEvent(TrackerRemovedEv{e.Sender, e.Err})
	} else {
		fmt.Printf("ANNOUNCE FAILED (retry in %v) -> [%v] BECAUSE: %v\n", retryIn, e.Sender.Announce.String(), e.Err)
		t.Sched.Schedule(
			TrackerTryAnnounceTsk{e.Sender, TRACKER_NONE},
			time.Now().Add(retryIn))
	}
}

//
// DISK EVENT HANDLERS
//

func (t *Torrent) handleDiskWriteSuccessfulEvent(e DiskWriteSuccessfulEv) {
	// fmt.Printf("WRITTEN (%v:%v) TO DISK\n", e.PieceIdx, e.BlockIdx)
	t.Picker.setBlockState(e.PieceIdx, e.BlockIdx, BLOCK_HAVE)

	if t.Picker.isPieceComplete(e.PieceIdx) {
		t.SignalEvent(PieceCompletedEv{e.PieceIdx})
	}
}

func (t *Torrent) handleDiskWriteFailedEvent(e DiskWriteFailedEv) {
	fmt.Println("DISK WRITE (%v:%v) FAILED -> %v", e.PieceIdx, e.BlockIdx, e.Err)
	t.Picker.removeBlock(e.PieceIdx, e.BlockIdx)
}

func (t *Torrent) handleDiskReadSuccessfulEvent(e DiskReadSuccessfulEv) {
	fmt.Printf("READ (%v:%v) FROM DISK\n", e.PieceIdx, e.BlockIdx)
	peer, ok := t.ActivePeers[e.RequestedFrom]
	if !ok {
		return
	}
	peer.Conn.SendBlock(e.PieceIdx, e.BlockIdx*t.DiskMan.BlockSize, e.Data)
}

func (t *Torrent) handleDiskHashSuccessfulEvent(e DiskHashSuccessfulEv) {
	// fmt.Println("HASH CHECK PASSED")
	t.Picker.setPieceState(e.PieceIdx, PIECE_HAVE)
	t.Picker.deletePieceBlockData(e.PieceIdx)
	t.bitfield.Set(uint(e.PieceIdx))
	t.Downloaded++
	t.Left--

	for _, peer := range t.ActivePeers {
		go peer.Conn.SendHave(e.PieceIdx)
	}
}

func (t *Torrent) handleDiskHashFailedEvent(e DiskHashFailedEv) {
	fmt.Printf("HASH CHECK FAILED -> [%v] BECAUSE: %v\n", e.PieceIdx, e.Err)
	t.Picker.resetPiece(e.PieceIdx)
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
	if t.Info.FileMode() == multi {
		for _, file := range t.Info.Files {
			t.DiskMan.AddFile(file.Path, file.Length)
		}
	} else {
		t.DiskMan.AddFile(t.Info.Name, *t.Info.Length)
	}

	trackers := []*Tracker{}
	if t.Info.AnnounceList != nil {
		for _, lst := range t.Info.AnnounceList {
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
