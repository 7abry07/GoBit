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

type RefillRequests struct{}

func (tsk PeerKeepAlive) IsTask()          {}
func (tsk PeerTryConnection) IsTask()      {}
func (tsk PeerCalculateStats) IsTask()     {}
func (ev RefillRequests) IsTask()          {}
func (tsk PeerKeepAlive) IsPeerTask()      {}
func (tsk PeerTryConnection) IsPeerTask()  {}
func (tsk PeerCalculateStats) IsPeerTask() {}
func (ev RefillRequests) IsPeerTask()      {}
