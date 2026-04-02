package protocol

import (
	"context"
	"fmt"
	"time"

	// "GoBit/internal/utils"
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
				// for pid, peer := range t.ActivePeers {
				// 	fmt.Printf("%v -> %v%v%v%v\n", pid,
				// 		utils.BoolToInt(peer.State.AmChoked),
				// 		utils.BoolToInt(peer.State.IsInteresting),
				// 		utils.BoolToInt(peer.State.WeRequested),
				// 		utils.BoolToInt(peer.State.WeReceived))
				// }

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
		t.SignalEvent(TrackerAdded{Sender: tracker})
	}
	now := time.Now()
	t.Sched.Schedule(ChokerTick{}, now.Add(time.Second*10))
	t.Sched.Schedule(OptimisticUnchokeTick{}, now.Add(time.Second*30))
}

func (t *Torrent) Stop() {
	t.cancel()
}

func (t *Torrent) Choke(p ActivePeer) {
	if p.State.IsChoked {
		return
	}
	p.Conn.SendChoked(true)
	p.State.IsChoked = true
}

func (t *Torrent) Unchoke(p ActivePeer) {
	if !p.State.IsChoked {
		return
	}
	p.Conn.SendChoked(false)
	p.State.IsChoked = false
}

func (t *Torrent) SetInteresting(p ActivePeer) {
	if p.State.IsInteresting {
		return
	}
	p.Conn.SendInterested(true)
	p.State.IsInteresting = true
}

func (t *Torrent) SetUninteresting(p ActivePeer) {
	if !p.State.IsInteresting {
		return
	}
	p.Conn.SendInterested(false)
	p.State.IsInteresting = false
}

func (t *Torrent) ClearOutstandingRequests(p ActivePeer) {
	for _, req := range p.State.PendingRequests {
		t.Picker.removeBlock(req.Idx, req.Begin/t.Info.BlockSize)
	}
	p.State.PendingRequests = []PeerRequest{}
}

func (t *Torrent) FillOutstandingRequest(p ActivePeer) {
	for len(p.State.PendingRequests) < 5 {
		newPieceIdx, ok := t.Picker.PickPiece(*p.State)
		if !ok {
			// fmt.Printf("NEW PIECE CANNOT BE REQUESTED\n")
			return
		}

		newBlockIdx, ok := t.Picker.getLowestFreeBlock(newPieceIdx)
		if !ok {
			// fmt.Printf("NEW BLOCK FROM PIECE %v CANNOT BE CHOSEN\n", newPieceIdx)
			return
		}

		t.Picker.setBlockState(newPieceIdx, newBlockIdx, BLOCK_REQUESTED)
		t.Picker.setPieceState(newPieceIdx, PIECE_DOWNLOADING)

		newReq := PeerRequest{
			Idx:    newPieceIdx,
			Begin:  newBlockIdx * t.Info.BlockSize,
			Length: t.Info.BlockSize,
		}

		p.State.PendingRequests = append(p.State.PendingRequests, newReq)
		p.Conn.SendRequest(newReq)
	}
}

func (t *Torrent) SignalEvent(e Event) {
	t.events <- e
}

func (t *Torrent) SignalTask(tsk Task) {
	t.tasks <- tsk
}
