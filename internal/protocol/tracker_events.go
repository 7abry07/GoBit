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

type TrackerAnnounced struct {
	Sender   Tracker
	Response TrackerAnnounceResponse
	Err      error
}

type TrackerScraped struct {
	Sender   Tracker
	Response TrackerScrapeResponse
	Err      error
}

func (TrackerAdded) IsEvent()            {}
func (TrackerRemoved) IsEvent()          {}
func (TrackerAnnounced) IsEvent()        {}
func (TrackerScraped) IsEvent()          {}
func (TrackerAdded) IsTrackerEvent()     {}
func (TrackerRemoved) IsTrackerEvent()   {}
func (TrackerScraped) IsTrackerEvent()   {}
func (TrackerAnnounced) IsTrackerEvent() {}
