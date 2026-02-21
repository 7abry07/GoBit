package protocol

import (
	"fmt"
	"math"
)

type Torrent struct {
	Info        TorrentFile
	PeerMan     *PeerManager
	PeerList    []*PeerConnection
	TrackerList []*Tracker

	NewPeer    chan *PeerConnection
	RemovePeer chan *PeerConnection
	PeerInbox  chan PeerMessage

	TrackerReady chan *Tracker
	TrackerInbox chan TrackerResult

	Pieces []uint8
}

func NewTorrent(file TorrentFile, peerMan *PeerManager) *Torrent {
	torrent := Torrent{}
	torrent.PeerMan = peerMan
	torrent.Info = file
	torrent.PeerList = []*PeerConnection{}
	torrent.TrackerList = []*Tracker{}
	torrent.NewPeer = make(chan *PeerConnection)
	torrent.RemovePeer = make(chan *PeerConnection)
	torrent.PeerInbox = make(chan PeerMessage)
	torrent.TrackerReady = make(chan *Tracker)
	torrent.TrackerInbox = make(chan TrackerResult)

	if file.AnnounceList != nil {
		for tier, lst := range *file.AnnounceList {
			for _, t := range lst {
				announce, err := NewTracker(t, uint8(tier), &torrent)
				if err == nil {
					torrent.TrackerList = append(torrent.TrackerList, announce)
				}
			}
		}
	} else {
		mainAnnounce, err := NewTracker(file.Announce, 0, &torrent)
		if err == nil {
			torrent.TrackerList = append(torrent.TrackerList, mainAnnounce)
		}
	}

	bitfieldSize := uint64(math.Ceil(float64(len(file.Pieces) / 20)))
	torrent.Pieces = make([]byte, bitfieldSize)

	return &torrent
}

func (t *Torrent) Start() {
	go t.trackerLoop()
	go t.peerLoop()

	for _, announce := range t.TrackerList {
		go t.AnnounceToTracker(announce, TrackerStarted)
	}
}

func (t *Torrent) AnnounceToTracker(tracker *Tracker, event TrackerEventType) {
	req := TrackerRequest{}
	req.Compact = 1
	req.Downloaded = 0
	req.Uploaded = 0
	req.Event = event
	req.Infohash = t.Info.InfoHash
	req.Kind = TrackerAnnounce
	req.Left = 0
	req.NoPID = 1
	req.Numwant = 50
	req.PeerID = t.PeerMan.PeerId
	req.Port = t.PeerMan.Port

	tracker.Out <- req
}

func (t *Torrent) peerLoop() {
	for {
		select {
		case p := <-t.NewPeer:
			{
				t.PeerList = append(t.PeerList, p)
			}
		case p := <-t.RemovePeer:
			{
				for i, val := range t.PeerList {
					if val == p {
						t.PeerList = append(t.PeerList[:i], t.PeerList[i+1:]...)
					}
				}
			}
		case mess := <-t.PeerInbox:
			{
				_ = mess
				// TODO
			}
		}
	}
}

func (t *Torrent) trackerLoop() {
	for {
		select {
		case ready := <-t.TrackerReady:
			{
				t.AnnounceToTracker(ready, TrackerNone)
			}
		case res := <-t.TrackerInbox:
			{
				if res.Err != nil {
				} else if res.Val.Failure != nil {
				} else {
					fmt.Printf("next announce in -> %v\n", res.Val.Interval/60)
					go t.PeerMan.DialPeers(t, res.Val.PeerList)
				}
			}
		}
	}
}
