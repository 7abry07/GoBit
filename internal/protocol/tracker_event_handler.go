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
	}
}

func (t *Torrent) handleTrackerAdded(e TrackerAdded) {
	t.TrackerList = append(t.TrackerList, e.Sender)

	t.Sched.Schedule(
		TrackerTryAnnounceTsk{e.Sender, TRACKER_STARTED},
		time.Now())

	fmt.Printf("TRACKER ADDED -> [%v]\n", e.Sender.Announce.Host)
}

func (t *Torrent) handleTrackerRemoved(e TrackerRemoved) {
	for i, val := range t.TrackerList {
		if val == e.Sender {
			t.TrackerList = append(t.TrackerList[:i], t.TrackerList[i+1:]...)
			fmt.Printf("TRACKER REMOVED -> [%v] BECAUSE: %v\n", e.Sender.Announce.Host, e.Cause)
		}
	}
}

func (t *Torrent) handleTrackerAnnounceSuccesful(e TrackerAnnounceSuccessful) {
	fmt.Printf("ANNOUNCED -> [%v] NEXT IN %v\n", e.Sender.Announce.String(), time.Second*time.Duration(e.Response.Interval))

	for _, entry := range e.Response.PeerList {
		peer := NewPeer(entry.IpPort)
		t.SignalEvent(PeerAdded{peer})
	}

	t.Sched.Schedule(
		TrackerTryAnnounceTsk{e.Sender, TRACKER_NONE},
		time.Now().Add(time.Second*time.Duration(e.Response.Interval)))
}

func (t *Torrent) handleTrackerAnnounceFailed(e TrackerAnnounceFailed) {
	e.Sender.FailureCnt++
	retryIn := (time.Minute) * time.Duration(math.Pow(2, float64(e.Sender.FailureCnt)))
	if retryIn > time.Hour*2 {
		fmt.Printf("ANNOUNCE FAILED (dropping tracker) -> [%v] BECAUSE: %v\n", e.Sender.Announce.String(), e.Err)
		t.SignalEvent(TrackerRemoved{e.Sender, e.Err})
	} else {
		fmt.Printf("ANNOUNCE FAILED (retry in %v) -> [%v] BECAUSE: %v\n", retryIn, e.Sender.Announce.String(), e.Err)
		t.Sched.Schedule(
			TrackerTryAnnounceTsk{e.Sender, TRACKER_NONE},
			time.Now().Add(retryIn))
	}
}
