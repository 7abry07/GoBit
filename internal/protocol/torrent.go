package protocol

import (
	"context"
	"fmt"
	"math"
)

type Torrent struct {
	ctx    context.Context
	cancel context.CancelFunc

	Ses         *Session
	PeerList    []*peerConnection
	TrackerList []*Tracker
	Info        TorrentFile

	NewPeer    chan *peerConnection
	RemovePeer chan *peerConnection
	PeerInbox  chan PeerMessage

	TrackerReady chan *Tracker
	TrackerInbox chan TrackerResult

	Download int64
	Upload   int64
	Left     int64
	Pieces   []uint8
}

func NewTorrent(file TorrentFile, ses *Session) *Torrent {
	torrent := Torrent{}
	torrent.ctx, torrent.cancel = context.WithCancel(ses.ctx)
	torrent.Ses = ses
	torrent.Info = file
	torrent.PeerList = []*peerConnection{}
	torrent.TrackerList = []*Tracker{}
	torrent.NewPeer = make(chan *peerConnection)
	torrent.RemovePeer = make(chan *peerConnection)
	torrent.PeerInbox = make(chan PeerMessage)
	torrent.TrackerReady = make(chan *Tracker)
	torrent.TrackerInbox = make(chan TrackerResult)
	torrent.Download = 0
	torrent.Upload = 0
	torrent.Left = int64(len(file.Pieces) / 20)
	bitfieldSize := uint64(math.Ceil(float64(len(file.Pieces) / 20)))
	torrent.Pieces = make([]byte, bitfieldSize)

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

	return &torrent
}

func (t *Torrent) loop() {
	for {
		select {
		case <-t.ctx.Done():
			{
				fmt.Println("TORRENT STOPPED")
				return
			}
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
				fmt.Printf("type -> %v\npayload -> %v\npeer -> %v\n", mess.Kind, mess.Payload, string(mess.Peer.Info.Pid.String()))
				// TODO
			}
		case ready := <-t.TrackerReady:
			{
				t.announceToTracker(ready, TrackerNone)
			}
		case res := <-t.TrackerInbox:
			{
				if res.Err != nil {
				} else if res.Val.Failure != nil {
				} else {
					fmt.Printf("next announce in -> %v\n", res.Val.Interval/60)
					go t.DialPeers(res.Val.PeerList)
				}
			}
		}
	}
}

func (t *Torrent) Start() {
	go t.loop()
	for _, announce := range t.TrackerList {
		go t.announceToTracker(announce, TrackerStarted)
	}
}

func (t *Torrent) Stop() {
	t.cancel()
}

func (t *Torrent) DialPeers(peers []Peer) {
	for _, p := range peers {
		go t.DialPeer(p)
	}
}

func (t *Torrent) DialPeer(p Peer) {
	conn, err := newPeerConnection(t, p, t.Ses.PeerID)
	if err != nil {
		fmt.Printf("CONNECTION FAILED -> %v\n", err.Error())
		return
	}

	fmt.Printf("CONNECTION SUCCESS -> %v\n", string(conn.Info.Pid.String()))
	t.NewPeer <- conn
}

func (t *Torrent) announceToTracker(tracker *Tracker, event TrackerEventType) {
	req := TrackerRequest{}
	req.Compact = 1
	req.Downloaded = t.Download
	req.Uploaded = t.Upload
	req.Event = event
	req.Infohash = t.Info.InfoHash
	req.Kind = TrackerAnnounce
	req.Left = t.Left
	req.NoPID = 1
	req.Numwant = 200
	req.PeerID = t.Ses.PeerID
	req.Port = t.Ses.Port

	tracker.Out <- req
	return
}
