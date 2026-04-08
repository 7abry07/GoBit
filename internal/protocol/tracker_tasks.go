package protocol

type TrackerTask interface {
	IsTrackerTask()
	Task
}

type TrackerTryAnnounce struct {
	Tracker Tracker
	Event   TrackerEventType
}

type TrackerTryScrape struct {
	Tracker Tracker
}

func (TrackerTryAnnounce) IsTask()        {}
func (TrackerTryAnnounce) IsTrackerTask() {}
func (TrackerTryScrape) IsTask()          {}
func (TrackerTryScrape) IsTrackerTask()   {}
