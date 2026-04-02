package protocol

import (
	"fmt"
)

func (t *Torrent) handleDiskEvent(e DiskEvent) {
	switch e := e.(type) {
	case DiskWriteSuccessful:
		t.handleDiskWriteSuccessful(e)
	case DiskWriteFailed:
		t.handleDiskWriteFailed(e)
	case DiskReadSuccessful:
		t.handleDiskReadSuccessful(e)
	case DiskHashPassed:
		t.handleDiskHashPassed(e)
	case DiskHashFailed:
		t.handleDiskHashFailed(e)
	}
}

func (t *Torrent) handleDiskWriteSuccessful(e DiskWriteSuccessful) {
	// fmt.Printf("WRITTEN (%v:%v) TO DISK\n", e.PieceIdx, e.BlockIdx)
	t.Picker.setBlockState(e.PieceIdx, e.BlockIdx, BLOCK_HAVE)

	if t.Picker.isPieceComplete(e.PieceIdx) {
		t.SignalEvent(PieceCompleted{e.PieceIdx})
	}
}

func (t *Torrent) handleDiskWriteFailed(e DiskWriteFailed) {
	fmt.Println("DISK WRITE (%v:%v) FAILED -> %v", e.PieceIdx, e.BlockIdx, e.Err)
	t.Picker.removeBlock(e.PieceIdx, e.BlockIdx)
}

func (t *Torrent) handleDiskReadSuccessful(e DiskReadSuccessful) {
	fmt.Printf("READ (%v:%v) FROM DISK\n", e.PieceIdx, e.BlockIdx)
	peer, ok := t.ActivePeers[e.RequestedFrom]
	if !ok {
		return
	}
	peer.Conn.SendBlock(e.PieceIdx, e.BlockIdx*t.DiskMan.BlockSize, e.Data)
}

func (t *Torrent) handleDiskHashPassed(e DiskHashPassed) {
	// fmt.Println("HASH CHECK PASSED")
	t.Picker.setPieceState(e.PieceIdx, PIECE_HAVE)
	t.Picker.deletePieceBlockData(e.PieceIdx)
	t.bitfield.Set(uint(e.PieceIdx))
	t.Downloaded++
	t.Left--

	for _, peer := range t.ActivePeers {
		if t.Picker.calculateInterested(*peer.State) {
			t.SetInteresting(peer)
		} else {
			t.SetUninteresting(peer)
		}
		peer.Conn.SendHave(e.PieceIdx)
	}
}

func (t *Torrent) handleDiskHashFailed(e DiskHashFailed) {
	fmt.Printf("HASH CHECK FAILED -> [%v] BECAUSE: %v\n", e.PieceIdx, e.Err)
	t.Picker.resetPiece(e.PieceIdx)
}
