package protocol

import (
	// "GoBit/internal/utils"
	"context"
	"fmt"
	"time"

	"github.com/bits-and-blooms/bitset"
)

type Torrent struct {
	ctx    context.Context
	cancel context.CancelFunc

	Info TorrentFile

	ActivePeers map[PeerID]ActivePeer
	Swarm       []*Peer
	TrackerList []*Tracker

	optimisticUnchoke *PeerID

	events chan Event
	tasks  chan Task

	bitfield   bitset.BitSet
	Downloaded int64
	Uploaded   int64
	Left       int64

	Ses     *Session
	Sched   *Scheduler
	Picker  *PiecePicker
	DiskMan *DiskManager
}

func NewTorrent(file TorrentFile, ses *Session) *Torrent {
	torrent := Torrent{}
	torrent.ctx, torrent.cancel = context.WithCancel(ses.ctx)
	torrent.Ses = ses
	torrent.Info = file
	torrent.Swarm = []*Peer{}
	torrent.ActivePeers = make(map[PeerID]ActivePeer)
	torrent.TrackerList = []*Tracker{}

	torrent.optimisticUnchoke = nil

	torrent.events = make(chan Event, 1024)
	torrent.tasks = make(chan Task, 1024)

	pieceCount := len(torrent.Info.Pieces) / 20

	torrent.bitfield = *bitset.New(uint(pieceCount))
	torrent.Left = int64(pieceCount)
	torrent.Downloaded = 0
	torrent.Uploaded = 0

	totalSize := uint64(0)
	if torrent.Info.FileMode() == multi {
		for _, file := range torrent.Info.Files {
			totalSize += file.Length
		}
	} else {
		totalSize = *torrent.Info.Length
	}

	torrent.DiskMan = NewDiskManager(&torrent, totalSize, uint32(pieceCount), torrent.Info.PieceSize, torrent.Info.BlockSize)
	torrent.Picker = NewPiecePicker(&torrent, totalSize, uint32(pieceCount), torrent.Info.PieceSize, torrent.Info.BlockSize)
	torrent.Sched = NewScheduler(&torrent)

	torrent.DiskMan.RootName = torrent.Info.Name
	torrent.DiskMan.DownloadDirectory = "/home/fabry/Downloads"

	return &torrent
}

func (t *Torrent) loop() {
	for {
		select {
		case <-t.ctx.Done():
			{
				fmt.Println("TORRENT STOPPED")
				t.Swarm = nil
				t.ActivePeers = nil
				t.Picker = nil

				for _, tracker := range t.TrackerList {
					tracker.SendAnnounce(
						t.Info.InfoHash,
						t.Downloaded,
						t.Uploaded,
						t.Left,
						TRACKER_STOPPED,
						t.Ses.PeerID,
						t.Ses.Port)
				}

				t.TrackerList = nil
				return
			}
		case task := <-t.tasks:
			{
				switch tsk := task.(type) {
				case PeerTask:
					t.handlePeerTask(tsk)
				case TrackerTask:
					t.handleTrackerTask(tsk)
				case ChokerTask:
					t.handleChokerTask(tsk)
				}
			}
		case event := <-t.events:
			{
				switch e := event.(type) {
				case PeerEvent:
					t.handlePeerEvent(e)
				case PeerMessage:
					t.handlePeerMessage(e)
				case TrackerEvent:
					t.handleTrackerEvent(e)
				case DiskEvent:
					t.handleDiskEvent(e)
				}
			}
		}
	}
}

func (t *Torrent) RescheduleBlock(req BlockRequest, badPeer PeerID) {
	t.Picker.removeBlock(req.Idx, req.Begin)
	for pid, peer := range t.ActivePeers {
		if peer.HasPiece(req.Idx) && pid != badPeer {
			// fmt.Printf("RESCHEDULING BLOCK (%v:%v:%v) from %v to %v\n", req.Idx, req.Begin, req.Length, badPeer, pid)
			peer.Request(req.Idx, req.Begin, req.Length)
			return
		}
	}
}

func (t *Torrent) Start() {
	if t.Info.FileMode() == multi {
		for _, file := range t.Info.Files {
			t.DiskMan.AddFile(file.Path, file.Length)
		}
	} else {
		t.DiskMan.AddFile(t.Info.Name, *t.Info.Length)
	}

	trackers := []*Tracker{}
	if t.Info.AnnounceList != nil {
		for _, lst := range t.Info.AnnounceList {
			for _, trackerUrl := range lst {
				announce, err := NewTracker(trackerUrl)
				if err == nil {
					trackers = append(trackers, announce)
				}
			}
		}
	} else {
		announce, err := NewTracker(*t.Info.Announce)
		if err == nil {
			trackers = append(trackers, announce)
		}
	}

	go t.loop()

	for _, tracker := range trackers {
		t.SignalEvent(TrackerAdded{tracker})
	}
	now := time.Now()
	t.Sched.Schedule(ChokerTick{}, now.Add(time.Second*10))
	t.Sched.Schedule(OptimisticUnchokeTick{}, now.Add(time.Second*30))

	// go func() {
	// 	ticker := time.NewTicker(time.Second * 5)
	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			for pid, peer := range t.ActivePeers {
	// 				// if !peer.State.AmChoked && len(peer.State.PendingRequests) == 0 {
	// 				// 	panic(fmt.Errorf("client unchoked but not requesting from %v", pid))
	// 				// }
	// 				fmt.Printf("%v -> state: %v%v | pendingReqs: %v | seed: %v\n", pid,
	// 					utils.BoolToInt(peer.State.AmChoked),
	// 					utils.BoolToInt(peer.State.IsInteresting),
	// 					len(peer.State.PendingRequests),
	// 					peer.State.IsSeed)
	// 			}
	// 		}
	// 	}
	// }()
}

func (t *Torrent) Stop() {
	t.cancel()
}

func (t *Torrent) SignalEvent(e Event) {
	t.events <- e
}

func (t *Torrent) SignalTask(tsk Task) {
	t.tasks <- tsk
}
