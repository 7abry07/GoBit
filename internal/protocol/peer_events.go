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
	BadPeer PeerID
	Idx     uint32
	Begin   uint32
	Length  uint32
}

type RescheduleBlock struct {
	BadPeer PeerID
	Idx     uint32
	Begin   uint32
	Length  uint32
}

func (PeerConnected) IsEvent()        {}
func (PeerDisconnected) IsEvent()     {}
func (PeerConnectionFailed) IsEvent() {}
func (PeerRemoved) IsEvent()          {}
func (PeerAdded) IsEvent()            {}
func (PieceCompleted) IsEvent()       {}
func (RequestTimeout) IsEvent()       {}
func (RescheduleBlock) IsEvent()      {}

func (PeerConnected) IsPeerEvent()        {}
func (PeerDisconnected) IsPeerEvent()     {}
func (PeerConnectionFailed) IsPeerEvent() {}
func (PeerRemoved) IsPeerEvent()          {}
func (PeerAdded) IsPeerEvent()            {}
func (PieceCompleted) IsPeerEvent()       {}
func (RequestTimeout) IsPeerEvent()       {}
func (RescheduleBlock) IsPeerEvent()      {}
