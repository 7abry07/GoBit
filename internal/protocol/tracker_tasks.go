package protocol

type TrackerTask interface {
	IsTrackerTask()
	Task
}

type TrackerTryAnnounceTsk struct {
	Tracker *Tracker
	Event   TrackerEventType
}

func (tsk TrackerTryAnnounceTsk) IsTask()        {}
func (tsk TrackerTryAnnounceTsk) IsTrackerTask() {}
