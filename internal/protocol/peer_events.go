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

func (ev PeerConnected) IsEvent()            {}
func (ev PeerDisconnected) IsEvent()         {}
func (ev PeerConnectionFailed) IsEvent()     {}
func (ev PeerRemoved) IsEvent()              {}
func (ev PeerAdded) IsEvent()                {}
func (ev PieceCompleted) IsEvent()           {}
func (ev RequestTimeout) IsEvent()           {}
func (ev PeerConnected) IsPeerEvent()        {}
func (ev PeerDisconnected) IsPeerEvent()     {}
func (ev PeerConnectionFailed) IsPeerEvent() {}
func (ev PeerRemoved) IsPeerEvent()          {}
func (ev PeerAdded) IsPeerEvent()            {}
func (ev PieceCompleted) IsPeerEvent()       {}
func (ev RequestTimeout) IsPeerEvent()       {}
