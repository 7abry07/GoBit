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

func (tsk PeerKeepAlive) IsTask()          {}
func (tsk PeerTryConnection) IsTask()      {}
func (tsk PeerCalculateStats) IsTask()     {}
func (tsk PeerKeepAlive) IsPeerTask()      {}
func (tsk PeerTryConnection) IsPeerTask()  {}
func (tsk PeerCalculateStats) IsPeerTask() {}
