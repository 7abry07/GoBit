package protocol

import (
	"fmt"
	"time"
)

func (t *Torrent) handleTorrentEvent(e TorrentEvent) {
	switch e := e.(type) {
	case TorrentStarted:
		t.handleTorrentStarted()
	case TorrentFinished:
		t.handleTorrentFinished()
	case PieceCompleted:
		t.handlePieceCompleted(e)
	case RequestTimeout:
		t.handleRequestTimeout(e)
	case RescheduleBlock:
		t.handleRescheduleBlock(e)
	}
}

func (t *Torrent) handleTorrentStarted() {
	t.Started = time.Now()
	fmt.Printf("TORRENT [%v] STARTED AT %v\n", t.Info.Name, t.Started.Format(time.RFC822))
	if t.Info.AnnounceList != nil {
		for _, lst := range t.Info.AnnounceList {
			for _, trackerUrl := range lst {
				switch trackerUrl.Scheme {
				case "https":
					fallthrough
				case "http":
					{
						http, err := NewHttpTracker(t, trackerUrl)
						if err == nil {
							t.SignalEvent(TrackerAdded{http})
						}
					}
				case "udp":
					{
						udp, err := NewUdpTracker(t, trackerUrl)
						if err == nil {
							t.SignalEvent(TrackerAdded{udp})
						}
						// TODO
					}
				}
			}
		}
	} else {
		switch t.Info.Announce.Scheme {
		case "https":
			fallthrough
		case "http":
			{
				http, err := NewHttpTracker(t, *t.Info.Announce)
				if err == nil {
					t.SignalEvent(TrackerAdded{http})
				}
			}
		case "udp":
			{
				udp, err := NewUdpTracker(t, *t.Info.Announce)
				if err == nil {
					t.SignalEvent(TrackerAdded{udp})
				}
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

func (t *Torrent) handlePieceCompleted(e PieceCompleted) {
	t.Picker.setPieceState(e.Idx, PIECE_COMPLETE)
	t.Picker.deletePieceBlockData(e.Idx)
	t.bitfield.Set(uint(e.Idx))

	fmt.Printf("PIECE COMPLETED -> %v [completed: %v] [%v : %v]\n", e.Idx, t.bitfield.Count(), t.Downloaded, t.Left)
	for _, peer := range t.ActivePeers {
		if t.Picker.CalculateInterested(t.bitfield, *peer.State) {
			peer.SetInteresting()
		} else {
			peer.SetUninteresting()
		}
		peer.Have(e.Idx)
	}

	if t.Left == 0 {
		t.SignalEvent(TorrentFinished{})
	}
}

func (t *Torrent) handleRequestTimeout(e RequestTimeout) {
	// fmt.Printf("REQUEST TIMEOUT -> %v [%v:%v:%v] \n", e.Req.To, e.Req.Idx, e.Req.Begin, e.Req.Length)
	p, ok := t.ActivePeers[e.BadPeer]
	if ok {
		p.Cancel(e.Idx, e.Begin, e.Length)
	}
	t.SignalEvent(RescheduleBlock{e.BadPeer, e.Idx, e.Begin, e.Length})
}

func (t *Torrent) handleRescheduleBlock(e RescheduleBlock) {
	t.Picker.BlocksToRequestInc()

	t.Picker.removeBlock(e.Idx, e.Begin)
	for pid, peer := range t.ActivePeers {
		if peer.HasPiece(e.Idx) && pid != e.BadPeer {
			// fmt.Printf("RESCHEDULING BLOCK (%v:%v:%v) from %v to %v\n", req.Idx, req.Begin, req.Length, badPeer, pid)
			peer.Request(e.Idx, e.Begin, e.Length)
			return
		}
	}
}
