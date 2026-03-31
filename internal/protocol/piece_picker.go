package protocol

import (
	"fmt"
	"slices"

	"github.com/bits-and-blooms/bitset"
)

type blockState uint8

const (
	BLOCK_REQUESTED blockState = iota
	BLOCK_RECEIVED
	BLOCK_HAVE
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

	pieces        []*pieceInfo
	pieceCount    uint32
	pieceSize     uint32
	blockSize     uint32
	blockPerPiece uint32

	lastPieceSize     uint32
	lastBlockPerPiece uint32
}

func NewPiecePicker(t *Torrent, totalSize uint64, pieceCount, pieceSize, blockSize uint32) *PiecePicker {
	p := PiecePicker{}
	p.pieces = make([]*pieceInfo, pieceCount)
	p.pieceCount = pieceCount
	p.pieceSize = pieceSize
	p.blockSize = blockSize
	p.blockPerPiece = pieceSize / blockSize
	p.lastPieceSize = uint32(totalSize % uint64(pieceSize))
	if p.lastPieceSize == 0 {
		p.lastPieceSize = pieceSize
	}
	p.lastBlockPerPiece = p.lastPieceSize / blockSize

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

func (p *PiecePicker) PickPiece(peer ActivePeerState) (uint32, bool) {
	availablePieces := []*pieceInfo{}

	for i := range peer.Pieces.EachSet() {
		if p.pieces[i].state != PIECE_HAVE && p.pieces[i].state != PIECE_COMPLETE {
			availablePieces = append(availablePieces, p.pieces[i])
		}
	}

	if len(availablePieces) == 0 {
		return 0, false
	}

	interestingPiece := availablePieces[0]
	for _, piece := range availablePieces {
		if piece.priority > interestingPiece.priority {
			interestingPiece = piece
		} else if piece.availability > interestingPiece.availability {
			interestingPiece = piece
		}
	}

	return interestingPiece.idx, true
}

func (p *PiecePicker) getLowestFreeBlock(pieceIdx uint32) (uint32, bool) {
	piece := p.pieces[pieceIdx]

	if len(piece.blocks) > int(p.GetBlocksPerPiece(pieceIdx))-1 {
		return 0, false
	}

	slices.SortFunc(piece.blocks, func(a, b *blockInfo) int {
		if a.idx < b.idx {
			return -1
		} else if a.idx > b.idx {
			return 1
		} else {
			return 0
		}
	})

	for i := range uint32(len(piece.blocks)) {
		if i != piece.blocks[i].idx {
			return i, true
		}
	}

	return uint32(len(piece.blocks)), true
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
		blockIdx, state,
	})
}

func (p *PiecePicker) resetPiece(pieceIdx uint32) {
	piece := p.pieces[pieceIdx]
	piece.state = PIECE_DONT_HAVE
	piece.blocks = nil
}

func (p *PiecePicker) deletePieceBlockData(pieceIdx uint32) {
	piece := p.pieces[pieceIdx]
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
		if block.state == BLOCK_HAVE {
			receivedBlocks = append(receivedBlocks, block)
		}
	}

	return len(receivedBlocks) == int(p.GetBlocksPerPiece(pieceIdx))
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
	if uint(len(p.pieces)) != bf.Len() {
		panic("")
	}
	for i := range bf.EachSet() {
		p.pieces[i].availability++
	}
}

func (p *PiecePicker) DecRefBitfield(bf *bitset.BitSet) {
	if uint(len(p.pieces)) != bf.Len() {
		if p.pieceCount != uint32(bf.Len()) {
			panic("")
		}
		for i := range bf.EachSet() {
			p.pieces[i].availability--
		}
	}
}

func (p *PiecePicker) Piority(idx uint32) piecePriority {
	return p.pieces[idx].priority
}

func (p *PiecePicker) GetPieceLength(idx uint32) uint32 {
	if idx == p.pieceCount-1 {
		return p.lastPieceSize
	} else {
		return p.pieceSize
	}
}

func (p *PiecePicker) GetBlocksPerPiece(idx uint32) uint32 {
	if idx == p.pieceCount-1 {
		return p.lastBlockPerPiece
	} else {
		return p.blockPerPiece
	}
}
