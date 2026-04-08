package protocol

import (
	"fmt"
	"math"
	"time"
)

func (t *Torrent) handleTrackerEvent(e TrackerEvent) {
	switch e := e.(type) {
	case TrackerAdded:
		t.handleTrackerAdded(e)
	case TrackerRemoved:
		t.handleTrackerRemoved(e)
	case TrackerAnnounceSuccessful:
		t.handleTrackerAnnounceSuccesful(e)
	case TrackerAnnounceFailed:
		t.handleTrackerAnnounceFailed(e)
	case TrackerScrapeSuccessful:
		t.handleTrackerScrapeSuccessful(e)
	case TrackerScrapeFailed:
		t.handleTrackerScrapeFailed(e)
	}
}

func (t *Torrent) handleTrackerAdded(e TrackerAdded) {
	t.TrackerList = append(t.TrackerList, e.Sender)
	t.Sched.Schedule(TrackerTryAnnounce{e.Sender, TRACKER_STARTED}, time.Now())
	fmt.Printf("TRACKER ADDED -> [%v]\n", e.Sender.GetHost())
}

func (t *Torrent) handleTrackerRemoved(e TrackerRemoved) {
	for i, tracker := range t.TrackerList {
		if tracker == e.Sender {
			t.TrackerList = append(t.TrackerList[:i], t.TrackerList[i+1:]...)
			fmt.Printf("TRACKER REMOVED -> [%v] %v\n", e.Sender.GetHost(), e.Cause)
			tracker.Stop()
		}
	}
}

func (t *Torrent) handleTrackerAnnounceSuccesful(e TrackerAnnounceSuccessful) {
	fmt.Printf("ANNOUNCED -> [%v] NEXT IN %v\n", e.Sender.GetHost(), time.Second*time.Duration(e.Response.Interval))

	for _, entry := range e.Response.PeerList {
		peer := NewPeer(entry.IpPort)
		t.SignalEvent(PeerAdded{peer})
	}

	t.Sched.Schedule(
		TrackerTryAnnounce{e.Sender, TRACKER_NONE},
		time.Now().Add(time.Second*time.Duration(e.Response.Interval)))
}

func (t *Torrent) handleTrackerAnnounceFailed(e TrackerAnnounceFailed) {
	e.Sender.Failure()
	retryIn := time.Second * time.Duration(15*math.Pow(2, float64(e.Sender.FailedCount())))
	if retryIn > time.Hour*2 {
		fmt.Printf("ANNOUNCE FAILED (dropping tracker) -> [%v] BECAUSE: %v\n", e.Sender.GetHost(), e.Err)
		t.SignalEvent(TrackerRemoved{e.Sender, e.Err})
	} else {
		// fmt.Printf("ANNOUNCE FAILED (retry in %v) -> [%v] BECAUSE: %v\n", retryIn, e.Sender.GetHost(), e.Err)
		t.Sched.Schedule(
			TrackerTryAnnounce{e.Sender, TRACKER_NONE},
			time.Now().Add(retryIn))
	}
}

func (t *Torrent) handleTrackerScrapeSuccessful(e TrackerScrapeSuccessful) {}
func (t *Torrent) handleTrackerScrapeFailed(e TrackerScrapeFailed)         {}
