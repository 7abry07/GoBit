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
				switch trackerUrl.Scheme {
				case "http":
					{
						http, err := NewHttpTracker(t, trackerUrl)
						if err == nil {
							t.SignalEvent(TrackerAdded{http})
						}
					}
				case "udp":
					{
						// TODO
					}
				}

			}
		}
	} else {
		switch t.Info.Announce.Scheme {
		case "http":
			{
				http, err := NewHttpTracker(t, *t.Info.Announce)
				if err == nil {
					t.SignalEvent(TrackerAdded{http})
				}
			}
		case "udp":
			{
				// TODO
			}
		}
	}

	t.Sched.Schedule(ChokerTick{}, t.Started.Add(time.Second*10))
	t.Sched.Schedule(OptimisticUnchokeTick{}, t.Started.Add(time.Second*30))
}

func (t *Torrent) handleTorrentFinished() {
	fmt.Printf("TORRENT [%v] FINISHED IN %v\n", t.Info.Name, time.Since(t.Started).Truncate(time.Second))
	t.Seeding = true
	for pid, peer := range t.ActivePeers {
		if peer.State.IsSeed {
			t.SignalEvent(PeerRemoved{peer.Conn.Peer, Peer_redundant})
			t.SignalEvent(PeerDisconnected{pid, Peer_redundant})
		}
	}
}
