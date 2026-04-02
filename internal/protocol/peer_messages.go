package protocol

import (
	"github.com/bits-and-blooms/bitset"
)

type PeerMessage interface {
	IsPeerMessage()
	Event
}

type PeerChoke struct {
	Sender PeerID
	Value  bool
}

type PeerInterested struct {
	Sender PeerID
	Value  bool
}

type PeerHave struct {
	Sender PeerID
	Idx    uint32
}

type PeerBitfield struct {
	Sender   PeerID
	Bitfield *bitset.BitSet
}

type PeerRequest struct {
	Sender PeerID
	Idx    uint32
	Begin  uint32
	Length uint32
}

type PeerPiece struct {
	Sender PeerID
	Idx    uint32
	Begin  uint32
	Block  []byte
}

type PeerCancel struct {
	Sender PeerID
	Idx    uint32
	Begin  uint32
	Length uint32
}

func (ev PeerChoke) IsEvent()      {}
func (ev PeerInterested) IsEvent() {}
func (ev PeerHave) IsEvent()       {}
func (ev PeerBitfield) IsEvent()   {}
func (ev PeerRequest) IsEvent()    {}
func (ev PeerPiece) IsEvent()      {}
func (ev PeerCancel) IsEvent()     {}

func (ev PeerChoke) IsPeerMessage()      {}
func (ev PeerInterested) IsPeerMessage() {}
func (ev PeerHave) IsPeerMessage()       {}
func (ev PeerBitfield) IsPeerMessage()   {}
func (ev PeerRequest) IsPeerMessage()    {}
func (ev PeerPiece) IsPeerMessage()      {}
func (ev PeerCancel) IsPeerMessage()     {}
