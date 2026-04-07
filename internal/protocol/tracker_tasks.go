package protocol

type TrackerTask interface {
	IsTrackerTask()
	Task
}

type TrackerTryAnnounce struct {
	Tracker Tracker
	Event   TrackerEventType
}

func (TrackerTryAnnounce) IsTask()        {}
func (TrackerTryAnnounce) IsTrackerTask() {}
