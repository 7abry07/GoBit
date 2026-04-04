package protocol

type PeerEvent interface {
	IsPeerEvent()
	Event
}

type PeerConnected struct {
	Sender   *PeerConnection
	Attempts int
}

type PeerDisconnected struct {
	Sender PeerID
	Cause  error
}

type PeerConnectionFailed struct {
	Sender *Peer
	Err    error
}

type PeerAdded struct {
	Sender *Peer
}

type PeerRemoved struct {
	Sender *Peer
	Cause  error
}

type PieceCompleted struct {
	Idx uint32
}

type RequestTimeout struct {
	Req BlockRequest
}

type RescheduleBlock struct {
	BadPeer PeerID
	Idx     uint32
	Begin   uint32
	Length  uint32
}

func (ev PeerConnected) IsEvent()            {}
func (ev PeerDisconnected) IsEvent()         {}
func (ev PeerConnectionFailed) IsEvent()     {}
func (ev PeerRemoved) IsEvent()              {}
func (ev PeerAdded) IsEvent()                {}
func (ev PieceCompleted) IsEvent()           {}
func (ev RequestTimeout) IsEvent()           {}
func (ev RescheduleBlock) IsEvent()          {}
func (ev PeerConnected) IsPeerEvent()        {}
func (ev PeerDisconnected) IsPeerEvent()     {}
func (ev PeerConnectionFailed) IsPeerEvent() {}
func (ev PeerRemoved) IsPeerEvent()          {}
func (ev PeerAdded) IsPeerEvent()            {}
func (ev PieceCompleted) IsPeerEvent()       {}
func (ev RequestTimeout) IsPeerEvent()       {}
func (ev RescheduleBlock) IsPeerEvent()      {}
