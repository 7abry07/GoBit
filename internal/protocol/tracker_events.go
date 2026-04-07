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
	Response TrackerResponse
}

type TrackerAnnounceFailed struct {
	Sender Tracker
	Err    error
}

func (TrackerAdded) IsEvent()                     {}
func (TrackerRemoved) IsEvent()                   {}
func (TrackerAnnounceFailed) IsEvent()            {}
func (TrackerAnnounceSuccessful) IsEvent()        {}
func (TrackerAdded) IsTrackerEvent()              {}
func (TrackerRemoved) IsTrackerEvent()            {}
func (TrackerAnnounceFailed) IsTrackerEvent()     {}
func (TrackerAnnounceSuccessful) IsTrackerEvent() {}
