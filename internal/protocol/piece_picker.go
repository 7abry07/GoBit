package protocol

import (
	"GoBit/internal/utils"
)

type blockState uint8
type pieceState uint8
type piecePriority uint8

const (
	REQUESTED blockState = iota
	RECEIVED
)

const (
	LOW piecePriority = iota
	NORMAL
	HIGH
)

const (
	DONT_HAVE pieceState = iota
	DOWNLOADING
	HAVE
)

type blockInfo struct {
	begin uint64
	state blockState
}

type pieceInfo struct {
	blocks []blockInfo

	availability uint8
	priority     piecePriority
	state        pieceState
}

type PiecePicker struct {
	pieces []pieceInfo

	blockLength uint32
	pieceCount  uint32
}

func NewPiecePicker(t *Torrent, blockLength uint32) *PiecePicker {
	p := PiecePicker{}
	p.blockLength = blockLength
	p.pieceCount = uint32(len(t.Info.Pieces) / 20)
	p.pieces = make([]pieceInfo, p.pieceCount)

	for _, piece := range p.pieces {
		piece.availability = 0
		piece.priority = NORMAL
		piece.state = DONT_HAVE

		piece.blocks = nil
	}

	return &p
}

func (p *PiecePicker) PickPiece(peer *Peer) (pieceInfo, bool) {
	if peer.State != CONNECTED {
		return pieceInfo{}, false
	}

	if peer.Conn.IsChoked {
		return pieceInfo{}, false
	}
	interestingPiece := pieceInfo{}
	availablePieces := []pieceInfo{}

	for i := range p.pieceCount {
		if peer.Pieces.IsSet(i) {
			availablePieces = append(availablePieces, p.pieces[i])
		}
	}
	interestingPiece = availablePieces[0]
	for _, piece := range availablePieces {
		if piece.priority > interestingPiece.priority {
			interestingPiece = piece
		} else if piece.availability > interestingPiece.availability {
			interestingPiece = piece
		}
	}

	return interestingPiece, true
}

func (p *PiecePicker) GetBitfield() *utils.Bitfield {
	bf := utils.NewBitfield(p.pieceCount)
	for i, piece := range p.pieces {
		if piece.state == HAVE {
			bf.Set(uint32(i), true)
		} else {
			bf.Set(uint32(i), false)
		}
	}
	return bf
}

func (p *PiecePicker) Piority(idx uint32) piecePriority {
	return p.pieces[idx].priority
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

func (p *PiecePicker) IncRefBitfield(bf *utils.Bitfield) bool {
	if p.pieceCount != bf.Count() {
		return false
	}
	for i := range p.pieceCount {
		if bf.IsSet(i) {
			p.pieces[i].availability++
		}
	}
	return true
}

func (p *PiecePicker) DecRefBitfield(bf *utils.Bitfield) bool {
	if p.pieceCount != bf.Count() {
		return false
	}
	for i := range p.pieceCount {
		if bf.IsSet(i) {
			p.pieces[i].availability--
		}
	}
	return true
}

func (p *PiecePicker) calculateInterested(peer *Peer) bool {
	for i, piece := range p.pieces {
		if piece.state == DONT_HAVE && peer.Pieces.IsSet(uint32(i)) {
			return true
		}
	}
	return false
}
