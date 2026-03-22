package protocol

import "GoBit/internal/utils"

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
	Idx    int
}

type PeerBitfieldEv struct {
	Sender   PeerID
	Bitfield *utils.Bitfield
}

type PeerRequestEv struct {
	Sender PeerID
	Idx    int
	Begin  int
	Length int
}

type PeerPieceEv struct {
	Sender PeerID
	Idx    int
	Begin  int
	Block  []byte
}

type PeerCancelEv struct {
	Sender PeerID
	Idx    int
	Begin  int
	Length int
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
