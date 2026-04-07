package protocol

type Tracker interface {
	Announce(TrackerEventType)
	Scrape()
	Failure()
	FailedCount() int
	GetHost() string
}
