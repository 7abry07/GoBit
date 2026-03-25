package protocol

import (
	"github.com/bits-and-blooms/bitset"
)

type Event interface {
	IsEvent()
}

//
// PEER EVENTS
//

type PeerConnectedEv struct {
	Sender   *PeerConnection
	Attempts int
}

type PeerDisconnectedEv struct {
	Sender PeerID
	Cause  error
}

type PeerAddedEv struct {
	Sender *Peer
}

type PeerRemovedEv struct {
	Sender *Peer
	Cause  error
}

type PeerChokeEv struct {
	Sender PeerID
	Value  bool
}

type PeerInterestedEv struct {
	Sender PeerID
	Value  bool
}

type PeerHaveEv struct {
	Sender PeerID
	Idx    uint32
}

type PeerBitfieldEv struct {
	Sender   PeerID
	Bitfield *bitset.BitSet
}

type PeerRequestEv struct {
	Sender PeerID
	Idx    uint32
	Begin  uint32
	Length uint32
}

type PeerPieceEv struct {
	Sender PeerID
	Idx    uint32
	Begin  uint32
	Block  []byte
}

type PeerCancelEv struct {
	Sender PeerID
	Idx    uint32
	Begin  uint32
	Length uint32
}

func (ev PeerConnectedEv) IsEvent()    {}
func (ev PeerDisconnectedEv) IsEvent() {}
func (ev PeerRemovedEv) IsEvent()      {}
func (ev PeerAddedEv) IsEvent()        {}
func (ev PeerChokeEv) IsEvent()        {}
func (ev PeerInterestedEv) IsEvent()   {}
func (ev PeerHaveEv) IsEvent()         {}
func (ev PeerBitfieldEv) IsEvent()     {}
func (ev PeerRequestEv) IsEvent()      {}
func (ev PeerPieceEv) IsEvent()        {}
func (ev PeerCancelEv) IsEvent()       {}

//
// TRACKER EVENTS
//

type TrackerAddedEv struct {
	Sender *Tracker
}

type TrackerRemovedEv struct {
	Sender *Tracker
	Cause  error
}

func (ev TrackerAddedEv) IsEvent()   {}
func (ev TrackerRemovedEv) IsEvent() {}
