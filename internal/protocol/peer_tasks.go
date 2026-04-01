package protocol

type PeerTask interface {
	IsPeerTask()
	Task
}

type PeerKeepAliveTsk struct {
	Receiver PeerID
}

type PeerCalculateStatsTsk struct {
	Receiver PeerID
}

type PeerTryConnectionTsk struct {
	Peer *Peer
}

func (tsk PeerKeepAliveTsk) IsTask()          {}
func (tsk PeerTryConnectionTsk) IsTask()      {}
func (tsk PeerCalculateStatsTsk) IsTask()     {}
func (tsk PeerKeepAliveTsk) IsPeerTask()      {}
func (tsk PeerTryConnectionTsk) IsPeerTask()  {}
func (tsk PeerCalculateStatsTsk) IsPeerTask() {}
