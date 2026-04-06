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
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		return
	}
	peer.State.AmChoked = e.Value

	if !e.Value {
		// fmt.Printf("UNCHOKED -> %v\n", e.Sender)
		if peer.State.IsInteresting {
			peer.FillOutstandingRequest()
		}
	} else {
		// fmt.Printf("CHOKED -> %v\n", e.Sender)
		peer.ClearOutstandingRequests()
	}
}

func (t *Torrent) handlePeerInterested(e PeerInterested) {
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		return
	}
	// fmt.Printf("INTERESTED (%v) -> %v\n", e.Value, e.Sender)
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

	if peer.State.Pieces.All() {
		peer.State.IsSeed = true
	}

	if !t.bitfield.Test(uint(e.Idx)) {
		peer.SetInteresting()
	} else {
		peer.SetUninteresting()
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

	if e.Bitfield.All() {
		peer.State.IsSeed = true
	}

	if t.Picker.CalculateInterested(t.bitfield, *peer.State) {
		peer.SetInteresting()
	} else {
		peer.SetUninteresting()
	}
}

func (t *Torrent) handlePeerRequest(e PeerRequest) {
	fmt.Printf("REQUEST -> [%v]\n", e.Sender)
	t.DiskMan.EnqueueJob(DiskReadJob{
		e.Sender, e.Idx, e.Begin, e.Length,
	})
}

func (t *Torrent) handlePeerPiece(e PeerPiece) {
	peer, ok := t.ActivePeers[e.Sender]
	if !ok {
		return
	}
	ok = peer.RemoveRequest(e.Idx, e.Begin, uint32(len(e.Block)))
	if !ok {
		return
	}

	// fmt.Printf("PIECE (%v:%v) -> %v\n", e.Idx, e.Begin, e.Sender)
	peer.State.TotalUploaded += len(e.Block)
	t.Picker.setBlockState(e.Idx, e.Begin, BLOCK_RECEIVED)
	t.DiskMan.EnqueueJob(DiskWriteJob{e.Idx, e.Begin, e.Block})
	if !peer.State.AmChoked {
		peer.FillOutstandingRequest()
	}
}

func (t *Torrent) handlePeerCancel(e PeerCancel) {
	fmt.Printf("CANCEL (%v:%v-%v) -> [%v]\n", e.Sender, e.Idx, e.Begin, e.Length)
	// TODO
}
