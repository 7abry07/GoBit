package protocol

func (t *Torrent) handleTrackerTask(tsk TrackerTask) {
	switch tsk := tsk.(type) {
	case TrackerTryAnnounce:
		t.handleTrackerTryAnnounce(tsk)
	}
}

func (t *Torrent) handleTrackerTryAnnounce(tsk TrackerTryAnnounce) {
	ih := t.Info.InfoHash
	d := t.Downloaded
	u := t.Uploaded
	l := t.Left

	tsk.Tracker.SendAnnounce(
		ih,
		d,
		u,
		l,
		tsk.Event,
		t.Ses.PeerID,
		t.Ses.Port)
}
