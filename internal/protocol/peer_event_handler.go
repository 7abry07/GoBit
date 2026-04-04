package protocol

import (
	"fmt"
	"github.com/bits-and-blooms/bitset"
	"math"
	"time"
)

func (t *Torrent) handlePeerEvent(e PeerEvent) {
	switch e := e.(type) {
	case PeerConnected:
		t.handlePeerConnected(e)
	case PeerDisconnected:
		t.handlePeerDisconnected(e)
	case PeerConnectionFailed:
		t.handlePeerConnectionFailed(e)
	case PeerAdded:
		t.handlePeerAdded(e)
	case PeerRemoved:
		t.handlePeerRemoved(e)
	case PieceCompleted:
		t.handlePieceCompleted(e)
	case RequestTimeout:
		t.handleRequestTimeout(e)
	}
}

func (t *Torrent) handlePeerConnected(e PeerConnected) {
	fmt.Printf("CONNECTED (attempts: %v) -> %v [%v]\n", e.Attempts, e.Sender.Pid.String(), len(t.ActivePeers))
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

	peer.Bitfield(t.bitfield)

	t.Sched.Schedule(
		PeerKeepAlive{e.Sender.Pid},
		time.Now().Add(time.Second*10))

	t.Sched.Schedule(
		PeerCalculateStats{e.Sender.Pid},
		time.Now().Add(time.Second))
}

func (t *Torrent) handlePeerDisconnected(e PeerDisconnected) {
	peer, ok := t.ActivePeers[e.Sender]
	if ok {
		delete(t.ActivePeers, peer.Conn.Pid)
		peer.Conn.Peer.PrevTotalDownloaded = peer.State.TotalDownloaded
		peer.Conn.Peer.PrevTotalUploaded = peer.State.TotalUploaded
		peer.Conn.Peer.Conn = nil
		t.Picker.DecRefBitfield(peer.State.Pieces)
		if peer.State.IsOptimistic {
			t.optimisticUnchoke = nil
		}

		fmt.Printf("DISCONNECTED -> %v BECAUSE: %v [%v]\n", e.Sender.String(), e.Cause, len(t.ActivePeers))
		peer.ClearOutstandingRequests()
	}
}

func (t *Torrent) handlePeerConnectionFailed(e PeerConnectionFailed) {
	e.Sender.FailureCnt++
	retryIn := (time.Minute) * time.Duration(math.Pow(2, float64(e.Sender.FailureCnt)))
	if retryIn > time.Minute*30 {
		// fmt.Printf("CONNECTION FAILED (dropping peer) -> [%v] BECAUSE: %v\n", e.Sender.Endpoint, e.Err)
		t.SignalEvent(PeerRemoved{e.Sender, e.Err})
	} else {
		// fmt.Printf("CONNECTION FAILED (retry in %v) -> [%v] BECAUSE: %v\n", retryIn, e.Sender.Endpoint, e.Err)
		t.Sched.Schedule(
			PeerTryConnection{e.Sender},
			time.Now().Add(retryIn))
	}
}

func (t *Torrent) handlePeerAdded(e PeerAdded) {
	t.Swarm = append(t.Swarm, e.Sender)
	// fmt.Printf("PEER ADDED -> %v\n", e.Sender.Endpoint.String())
	if e.Sender.Conn == nil {
		t.Sched.Schedule(
			PeerTryConnection{e.Sender},
			time.Now())
	}
}

func (t *Torrent) handlePeerRemoved(e PeerRemoved) {
	for i, val := range t.Swarm {
		if val == e.Sender {
			t.Swarm = append(t.Swarm[:i], t.Swarm[i+1:]...)
			fmt.Printf("PEER REMOVED BECAUSE: %v\n", e.Cause)
			return
		}
	}
}

func (t *Torrent) handlePieceCompleted(e PieceCompleted) {
	t.Picker.setPieceState(e.Idx, PIECE_COMPLETE)
	t.Picker.deletePieceBlockData(e.Idx)
	t.bitfield.Set(uint(e.Idx))
	t.Downloaded++
	t.Left--

	fmt.Printf("PIECE COMPLETED -> %v [%v]\n", e.Idx, t.Downloaded)
	for _, peer := range t.ActivePeers {
		if t.Picker.CalculateInterested(t.bitfield, *peer.State) {
			peer.SetInteresting()
		} else {
			peer.SetUninteresting()
		}
		peer.Have(e.Idx)
	}
}

func (t *Torrent) handleRequestTimeout(e RequestTimeout) {
	// fmt.Printf("REQUEST TIMEOUT -> %v [%v:%v:%v] \n", e.Req.To, e.Req.Idx, e.Req.Begin, e.Req.Length)
	p, ok := t.ActivePeers[e.Req.To]
	pieceIdx := e.Req.Idx
	blockOff := e.Req.Begin
	if ok {
		p.Cancel(pieceIdx, blockOff, e.Req.Length)
	}

	t.RescheduleBlock(e.Req, e.Req.To)
}
