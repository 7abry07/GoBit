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

type PeerConnectionFailedEv struct {
	Sender *Peer
	Err    error
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

type PieceCompletedEv struct {
	Idx uint32
}

func (ev PeerConnectedEv) IsEvent()        {}
func (ev PeerDisconnectedEv) IsEvent()     {}
func (ev PeerConnectionFailedEv) IsEvent() {}
func (ev PeerRemovedEv) IsEvent()          {}
func (ev PeerAddedEv) IsEvent()            {}
func (ev PeerChokeEv) IsEvent()            {}
func (ev PeerInterestedEv) IsEvent()       {}
func (ev PeerHaveEv) IsEvent()             {}
func (ev PeerBitfieldEv) IsEvent()         {}
func (ev PeerRequestEv) IsEvent()          {}
func (ev PeerPieceEv) IsEvent()            {}
func (ev PeerCancelEv) IsEvent()           {}
func (ev PieceCompletedEv) IsEvent()       {}

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

type TrackerAnnounceSuccessfulEv struct {
	Sender   *Tracker
	Response TrackerResponse
}

type TrackerAnnounceFailedEv struct {
	Sender *Tracker
	Err    error
}

func (ev TrackerAddedEv) IsEvent()   {}
func (ev TrackerRemovedEv) IsEvent() {}

func (ev TrackerAnnounceFailedEv) IsEvent()     {}
func (ev TrackerAnnounceSuccessfulEv) IsEvent() {}

//
// DISK EVENTS
//

type DiskWriteSuccessfulEv struct {
	PieceIdx uint32
	BlockIdx uint32
}

type DiskWriteFailedEv struct {
	PieceIdx uint32
	BlockIdx uint32
	Err      error
}

type DiskReadSuccessfulEv struct {
	RequestedFrom PeerID
	PieceIdx      uint32
	BlockIdx      uint32
	Data          []byte
}

// type DiskReadFailedEv struct {
// 	RequestedFrom PeerID
// 	PieceIdx      uint32
// 	BlockIdx      uint32
// 	Err           error
// }

type DiskHashSuccessfulEv struct {
	PieceIdx uint32
}

type DiskHashFailedEv struct {
	PieceIdx uint32
	Err      error
}

func (ev DiskWriteSuccessfulEv) IsEvent() {}
func (ev DiskWriteFailedEv) IsEvent()     {}

func (ev DiskReadSuccessfulEv) IsEvent() {}

// func (ev DiskReadFailedEv) IsEvent()     {}

func (ev DiskHashSuccessfulEv) IsEvent() {}
func (ev DiskHashFailedEv) IsEvent()     {}
