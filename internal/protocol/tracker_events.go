package protocol

type TrackerEvent interface {
	IsTrackerEvent()
	Event
}

type TrackerAdded struct {
	Sender *Tracker
}

type TrackerRemoved struct {
	Sender *Tracker
	Cause  error
}

type TrackerAnnounceSuccessful struct {
	Sender   *Tracker
	Response TrackerResponse
}

type TrackerAnnounceFailed struct {
	Sender *Tracker
	Err    error
}

func (ev TrackerAdded) IsEvent()                     {}
func (ev TrackerRemoved) IsEvent()                   {}
func (ev TrackerAnnounceFailed) IsEvent()            {}
func (ev TrackerAnnounceSuccessful) IsEvent()        {}
func (ev TrackerAdded) IsTrackerEvent()              {}
func (ev TrackerRemoved) IsTrackerEvent()            {}
func (ev TrackerAnnounceFailed) IsTrackerEvent()     {}
func (ev TrackerAnnounceSuccessful) IsTrackerEvent() {}
