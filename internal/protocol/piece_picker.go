package protocol

import (
	"crypto/sha1"
	"fmt"
	"slices"

	"github.com/bits-and-blooms/bitset"
)

type blockState uint8

const (
	BLOCK_REQUESTED blockState = iota
	BLOCK_RECEIVED
)

type pieceState uint8

const (
	PIECE_DONT_HAVE pieceState = iota
	PIECE_DOWNLOADING
	PIECE_COMPLETE
	PIECE_HAVE
)

type piecePriority uint8

const (
	PIECE_PRIORITY_LOW piecePriority = iota
	PIECE_PRIORITY_NORMAL
	PIECE_PRIORITY_HIGH
)

type blockInfo struct {
	idx   uint32
	state blockState
	data  []byte
}

type pieceInfo struct {
	blocks []*blockInfo

	idx          uint32
	availability uint8
	priority     piecePriority
	state        pieceState
}

type PiecePicker struct {
	torrent *Torrent

	pieces     []*pieceInfo
	pieceCount uint
}

func NewPiecePicker(t *Torrent) *PiecePicker {
	p := PiecePicker{}
	p.pieceCount = uint(len(t.Info.Pieces) / 20)
	p.pieces = make([]*pieceInfo, p.pieceCount)
	p.torrent = t

	for i, _ := range p.pieces {
		p.pieces[i] = &pieceInfo{
			nil,
			uint32(i),
			0,
			PIECE_PRIORITY_NORMAL,
			PIECE_DONT_HAVE,
		}
	}

	return &p
}

func (p *PiecePicker) PickPiece(peer ActivePeerState) uint32 {
	availablePieces := []*pieceInfo{}

	for i := range peer.Pieces.EachSet() {
		if p.pieces[i].state != PIECE_HAVE &&
			len(p.pieces[i].blocks) < 512 {
			availablePieces = append(availablePieces, p.pieces[i])
		}
	}

	interestingPiece := availablePieces[0]
	for _, piece := range availablePieces {
		if piece.priority > interestingPiece.priority {
			interestingPiece = piece
		} else if piece.availability > interestingPiece.availability {
			interestingPiece = piece
		}
	}

	return interestingPiece.idx
}

func (p *PiecePicker) getLowestFreeBlock(pieceIdx uint32) uint32 {
	piece := p.pieces[pieceIdx]
	slices.SortFunc(piece.blocks, func(a, b *blockInfo) int {
		if a.idx < b.idx {
			return -1
		} else if a.idx > b.idx {
			return 1
		} else {
			return 0
		}
	})

	for i := uint32(0); i < uint32(len(piece.blocks)); i++ {
		if i != piece.blocks[i].idx {
			return i
		}
	}

	if len(piece.blocks) > 512 {
		panic("")
	}

	return uint32(len(piece.blocks))
}

func (p *PiecePicker) setBlockState(pieceIdx uint32, blockIdx uint32, state blockState) {
	piece := p.pieces[pieceIdx]
	for _, b := range piece.blocks {
		if b.idx == blockIdx {
			b.state = state
			return
		}
	}
	piece.blocks = append(piece.blocks, &blockInfo{
		blockIdx, state, nil,
	})
}

func (p *PiecePicker) setBlockData(pieceIdx uint32, blockIdx uint32, data []byte) {
	piece := p.pieces[pieceIdx]
	for _, b := range piece.blocks {
		if b.idx == blockIdx {
			b.data = data
			return
		}
	}
	panic("setting data to non existing block")
}

func (p *PiecePicker) getPieceHash(pieceIdx uint32) []byte {
	pieceData := []byte{}
	piece := p.pieces[pieceIdx]
	if piece.state != PIECE_COMPLETE {
		panic("tried to compute hash of non complete piece")
	}

	slices.SortFunc(piece.blocks, func(a, b *blockInfo) int {
		if a.idx < b.idx {
			return -1
		} else if a.idx > b.idx {
			return 1
		} else {
			panic("")
		}
	})

	for _, block := range piece.blocks {
		pieceData = append(pieceData, block.data...)
	}

	hasher := sha1.New()
	hasher.Write(pieceData)
	return hasher.Sum([]byte{})
}

func (p *PiecePicker) resetPiece(pieceIdx uint32) {
	piece := p.pieces[pieceIdx]
	piece.state = PIECE_DONT_HAVE
	piece.blocks = nil
}

func (p *PiecePicker) setPieceState(pieceIdx uint32, state pieceState) {
	p.pieces[pieceIdx].state = state
}

func (p *PiecePicker) removeBlock(pieceIdx uint32, blockIdx uint32) {
	piece := p.pieces[pieceIdx]
	for i, block := range piece.blocks {
		if block.idx == blockIdx {
			piece.blocks = append(piece.blocks[:i], piece.blocks[i+1:]...)
			return
		}
	}
	panic(fmt.Errorf("non existing block removed (%v:%v)", pieceIdx, blockIdx))
}

func (p *PiecePicker) isPieceComplete(pieceIdx uint32) bool {
	piece := p.pieces[pieceIdx]
	receivedBlocks := []*blockInfo{}

	for _, block := range piece.blocks {
		if block.state == BLOCK_RECEIVED {
			receivedBlocks = append(receivedBlocks, block)
		}
	}

	return uint32(len(receivedBlocks))*p.torrent.Info.BlockLength == p.torrent.Info.PieceLength
}

func (p *PiecePicker) calculateInterested(peer ActivePeerState) bool {
	for i := range peer.Pieces.EachSet() {
		if p.pieces[i].state == PIECE_DONT_HAVE {
			return true
		}
	}

	return false
}

func (p *PiecePicker) SetPriority(idx uint32, prio piecePriority) {
	p.pieces[idx].priority = prio
}

func (p *PiecePicker) IncRef(idx uint32) {
	p.pieces[idx].availability++
}

func (p *PiecePicker) DecRef(idx uint32) {
	p.pieces[idx].availability--
}

func (p *PiecePicker) IncRefBitfield(bf *bitset.BitSet) {
	if p.pieceCount != bf.Len() {
		panic("")
	}
	for i := range bf.EachSet() {
		p.pieces[i].availability++
	}
}

func (p *PiecePicker) DecRefBitfield(bf *bitset.BitSet) {
	if p.pieceCount != bf.Len() {
		panic("")
	}
	for i := range bf.EachSet() {
		p.pieces[i].availability--
	}
}

func (p *PiecePicker) Piority(idx uint32) piecePriority {
	return p.pieces[idx].priority
}
