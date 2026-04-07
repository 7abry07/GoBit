package protocol

func (t *Torrent) handleTrackerTask(tsk TrackerTask) {
	switch tsk := tsk.(type) {
	case TrackerTryAnnounce:
		t.handleTrackerTryAnnounce(tsk)
	}
}

func (t *Torrent) handleTrackerTryAnnounce(tsk TrackerTryAnnounce) {
	tsk.Tracker.Announce(tsk.Event)
}
