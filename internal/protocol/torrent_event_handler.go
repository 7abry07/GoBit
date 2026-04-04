package protocol

import (
	"fmt"
	"time"
)

func (t *Torrent) handleTorrentEvent(e TorrentEvent) {
	switch e.(type) {
	case TorrentStarted:
		t.handleTorrentStarted()
	case TorrentFinished:
		t.handleTorrentFinished()
	}
}

func (t *Torrent) handleTorrentStarted() {
	t.Started = time.Now()
	fmt.Printf("TORRENT [%v] STARTED AT %v\n", t.Info.Name, t.Started.Format(time.RFC822))
	if t.Info.AnnounceList != nil {
		for _, lst := range t.Info.AnnounceList {
			for _, trackerUrl := range lst {
				announce, err := NewTracker(trackerUrl)
				if err == nil {
					t.SignalEvent(TrackerAdded{announce})
				}
			}
		}
	} else {
		announce, err := NewTracker(*t.Info.Announce)
		if err == nil {
			t.SignalEvent(TrackerAdded{announce})
		}
	}

	t.Sched.Schedule(ChokerTick{}, t.Started.Add(time.Second*10))
	t.Sched.Schedule(OptimisticUnchokeTick{}, t.Started.Add(time.Second*30))
}

func (t *Torrent) handleTorrentFinished() {
	fmt.Printf("TORRENT [%v] FINISHED IN %v\n", t.Info.Name, time.Now().Sub(t.Started).Truncate(time.Second))
}
