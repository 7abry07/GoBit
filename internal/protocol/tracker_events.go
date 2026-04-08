package protocol

type TrackerEvent interface {
	IsTrackerEvent()
	Event
}

type TrackerAdded struct {
	Sender Tracker
}

type TrackerRemoved struct {
	Sender Tracker
	Cause  error
}

type TrackerAnnounceSuccessful struct {
	Sender   Tracker
	Response TrackerAnnounceResponse
}

type TrackerAnnounceFailed struct {
	Sender Tracker
	Err    error
}

type TrackerScrapeSuccessful struct {
	Sender   Tracker
	Response TrackerScrapeResponse
}

type TrackerScrapeFailed struct {
	Sender Tracker
	Err    error
}

func (TrackerAdded) IsEvent()                     {}
func (TrackerRemoved) IsEvent()                   {}
func (TrackerAnnounceFailed) IsEvent()            {}
func (TrackerScrapeFailed) IsEvent()              {}
func (TrackerAnnounceSuccessful) IsEvent()        {}
func (TrackerScrapeSuccessful) IsEvent()          {}
func (TrackerAdded) IsTrackerEvent()              {}
func (TrackerRemoved) IsTrackerEvent()            {}
func (TrackerScrapeFailed) IsTrackerEvent()       {}
func (TrackerAnnounceFailed) IsTrackerEvent()     {}
func (TrackerAnnounceSuccessful) IsTrackerEvent() {}
func (TrackerScrapeSuccessful) IsTrackerEvent()   {}
