package protocol

import (
	"fmt"
)

func (t *Torrent) handleDiskEvent(e DiskEvent) {
	switch e := e.(type) {
	case DiskWriteFinished:
		t.handleDiskWriteFinished(e)
	case DiskReadFinished:
		t.handleDiskReadFinished(e)
	case DiskHashFinished:
		t.handleDiskHashFinished(e)
	}
}

func (t *Torrent) handleDiskWriteFinished(e DiskWriteFinished) {
	if e.Err != nil {
		fmt.Println("DISK WRITE (%v:%v) FAILED -> %v", e.PieceIdx, e.BlockOff, e.Err)
		t.Picker.removeBlock(e.PieceIdx, e.BlockOff)
	} else {
		// fmt.Printf("WRITTEN (%v:%v) TO DISK\n", e.PieceIdx, e.BlockOff)
		t.Picker.setBlockState(e.PieceIdx, e.BlockOff, BLOCK_HAVE)
		if t.Picker.isPieceComplete(e.PieceIdx) {
			t.DiskMan.EnqueueJob(DiskHashJob{e.PieceIdx})
		}
	}
}

func (t *Torrent) handleDiskReadFinished(e DiskReadFinished) {
	if e.Err != nil {
		fmt.Printf("READ FAILED (%v:%v) BECAUSE: %v\n", e.PieceIdx, e.BlockOff, e.Err)
	} else {
		fmt.Printf("READ (%v:%v) FROM DISK\n", e.PieceIdx, e.BlockOff)
		peer, ok := t.ActivePeers[e.RequestedFrom]
		if !ok || !peer.RequestCanceled(e.PieceIdx, e.BlockOff, uint32(len(e.Data))) {
			return
		}
		peer.Piece(e.PieceIdx, e.BlockOff, e.Data)
	}
}

func (t *Torrent) handleDiskHashFinished(e DiskHashFinished) {
	if e.Err != nil {
		fmt.Printf("HASH CHECK FAILED -> [%v] BECAUSE: %v\n", e.PieceIdx, e.Err)
		t.Picker.resetPiece(e.PieceIdx)
	} else {
		// fmt.Println("HASH CHECK PASSED")
		t.SignalEvent(PieceCompleted{e.PieceIdx})
	}
}
