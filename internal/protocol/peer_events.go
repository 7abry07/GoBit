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

func (PeerConnected) IsEvent()        {}
func (PeerDisconnected) IsEvent()     {}
func (PeerConnectionFailed) IsEvent() {}
func (PeerRemoved) IsEvent()          {}
func (PeerAdded) IsEvent()            {}

func (PeerConnected) IsPeerEvent()        {}
func (PeerDisconnected) IsPeerEvent()     {}
func (PeerConnectionFailed) IsPeerEvent() {}
func (PeerRemoved) IsPeerEvent()          {}
func (PeerAdded) IsPeerEvent()            {}
