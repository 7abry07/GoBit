package protocol

import (
	"context"
	"fmt"
	"time"

	"github.com/bits-and-blooms/bitset"
)

type Torrent struct {
	ctx    context.Context
	cancel context.CancelFunc

	Started time.Time
	Info    TorrentFile

	ActivePeers map[PeerID]ActivePeer
	TrackerList []Tracker
	Swarm       []*Peer

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

	Seeding bool
}

func NewTorrent(file TorrentFile, ses *Session) *Torrent {
	torrent := Torrent{}
	torrent.ctx, torrent.cancel = context.WithCancel(ses.ctx)
	torrent.Ses = ses
	torrent.Info = file
	torrent.Swarm = []*Peer{}
	torrent.ActivePeers = make(map[PeerID]ActivePeer)
	torrent.TrackerList = []Tracker{}

	torrent.optimisticUnchoke = nil

	torrent.events = make(chan Event, 1024)
	torrent.tasks = make(chan Task, 1024)

	pieceCount := len(torrent.Info.Pieces) / 20

	torrent.bitfield = *bitset.New(uint(pieceCount))
	torrent.Left = int64(pieceCount)
	torrent.Downloaded = 0
	torrent.Uploaded = 0
	torrent.Seeding = false

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

	if torrent.Info.FileMode() == multi {
		for _, file := range torrent.Info.Files {
			torrent.DiskMan.AddFile(file.Path, file.Length)
		}
	} else {
		torrent.DiskMan.AddFile(torrent.Info.Name, *torrent.Info.Length)
	}

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

				// for _, tracker := range t.TrackerList {
				// 	tracker.SendAnnounce(
				// 		t.Info.InfoHash,
				// 		t.Downloaded,
				// 		t.Uploaded,
				// 		t.Left,
				// 		TRACKER_STOPPED,
				// 		t.Ses.PeerID,
				// 		t.Ses.Port)
				// }

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
				case TorrentEvent:
					t.handleTorrentEvent(e)
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

func (t *Torrent) Start() {
	go t.loop()
	t.SignalEvent(TorrentStarted{})
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
