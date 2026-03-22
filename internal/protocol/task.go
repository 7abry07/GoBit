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

type PeerTryConnectionTsk struct {
	Peer *Peer
}

func (tsk PeerKeepAliveTsk) IsTask()     {}
func (tsk PeerTryConnectionTsk) IsTask() {}

//
// TRACKER TASKS
//

type TrackerNextAnnounceTsk struct {
	Tracker *Tracker
	Event   TrackerEventType
}

func (tsk TrackerNextAnnounceTsk) IsTask() {}
