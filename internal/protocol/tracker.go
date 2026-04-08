package protocol

type TrackerEventType int
type TrackerRequestKind int

const (
	TRACKER_NONE TrackerEventType = iota
	TRACKER_COMPLETED
	TRACKER_STARTED
	TRACKER_STOPPED
)

type Tracker interface {
	Announce(TrackerEventType)
	Scrape()
	Stop()
	Failure()
	ResetFailure()
	FailedCount() int
	GetHost() string
}
