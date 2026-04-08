package protocol

type TrackerEventType int
type TrackerRequestKind int

const (
	TRACKER_NONE TrackerEventType = iota
	TRACKER_COMPLETED
	TRACKER_STARTED
	TRACKER_STOPPED
)

const (
	TrackerAnnounce TrackerRequestKind = iota
	TrackerScrape
)

type Tracker interface {
	Announce(TrackerEventType)
	Scrape()
	Failure()
	FailedCount() int
	GetHost() string
}
