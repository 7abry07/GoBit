package protocol

type Task interface {
	IsTask()
}

//
// PEER TASKS
//

type PeerKeepAliveTsk struct {
	Receiver PeerID
}

type PeerCalculateStatsTsk struct {
	Receiver PeerID
}

type PeerTryConnectionTsk struct {
	Peer *Peer
}

type TrackerTryAnnounceTsk struct {
	Tracker *Tracker
	Event   TrackerEventType
}

type ChokerTsk struct{}
type OptimisticUnchokeTsk struct{}

func (tsk PeerKeepAliveTsk) IsTask()      {}
func (tsk PeerTryConnectionTsk) IsTask()  {}
func (tsk PeerCalculateStatsTsk) IsTask() {}

func (tsk TrackerTryAnnounceTsk) IsTask() {}

func (tsk ChokerTsk) IsTask()            {}
func (tsk OptimisticUnchokeTsk) IsTask() {}
