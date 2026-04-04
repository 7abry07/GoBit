package protocol

type PeerTask interface {
	IsPeerTask()
	Task
}

type PeerKeepAlive struct {
	Receiver PeerID
}

type PeerCalculateStats struct {
	Receiver PeerID
}

type PeerTryConnection struct {
	Peer *Peer
}

func (PeerKeepAlive) IsTask()          {}
func (PeerTryConnection) IsTask()      {}
func (PeerCalculateStats) IsTask()     {}
func (PeerKeepAlive) IsPeerTask()      {}
func (PeerTryConnection) IsPeerTask()  {}
func (PeerCalculateStats) IsPeerTask() {}
