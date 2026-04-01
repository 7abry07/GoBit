package protocol

import (
	"fmt"
)

func (t *Torrent) handlePeerMessage(m PeerMessage) {
	switch m := m.(type) {
	case PeerChoke:
		t.handlePeerChoke(m)
	case PeerInterested:
		t.handlePeerInterested(m)
	case PeerHave:
		t.handlePeerHave(m)
	case PeerBitfield:
		t.handlePeerBitfield(m)
	case PeerRequest:
		t.handlePeerRequest(m)
	case PeerPiece:
		t.handlePeerPiece(m)
	case PeerCancel:
		t.handlePeerCancel(m)
	}
}

func (t *Torrent) handlePeerChoke(e PeerChoke) {
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
		for len(peer.State.PendingRequests) < 5 {
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

func (t *Torrent) handlePeerInterested(e PeerInterested) {
	// fmt.Printf("UN/INTERESTED -> %v\n", e.Sender)
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		return
	}
	peer.State.AmInteresting = e.Value
}

func (t *Torrent) handlePeerHave(e PeerHave) {
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

func (t *Torrent) handlePeerBitfield(e PeerBitfield) {
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

func (t *Torrent) handlePeerRequest(e PeerRequest) {
	fmt.Printf("REQUEST -> [%v]\n", e.Sender)
	t.DiskMan.EnqueueJob(DiskReadJob{
		e.Sender, e.Idx, e.Begin / t.Info.BlockSize, e.Length,
	})
}

func (t *Torrent) handlePeerPiece(e PeerPiece) {
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

func (t *Torrent) handlePeerCancel(e PeerCancel) {
	// TODO
}
