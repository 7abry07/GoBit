package protocol

import (
	"GoBit/internal/utils"
	"context"
	"encoding/binary"
	"fmt"
)

type Torrent struct {
	ctx    context.Context
	cancel context.CancelFunc

	Ses         *Session
	PeerList    []*peerConnection
	TrackerList []*Tracker
	Info        TorrentFile

	newPeer   chan *peerConnection
	remPeer   chan *peerConnection
	peerInbox chan peerMessage

	trackerReady chan *Tracker
	trackerInbox chan TrackerResult

	Download int64
	Upload   int64
	Left     int64

	Picker *PiecePicker
}

func NewTorrent(file TorrentFile, ses *Session) *Torrent {
	torrent := Torrent{}
	torrent.ctx, torrent.cancel = context.WithCancel(ses.ctx)
	torrent.Ses = ses
	torrent.Info = file
	torrent.PeerList = []*peerConnection{}
	torrent.TrackerList = []*Tracker{}
	torrent.newPeer = make(chan *peerConnection)
	torrent.remPeer = make(chan *peerConnection)
	torrent.peerInbox = make(chan peerMessage)
	torrent.trackerReady = make(chan *Tracker)
	torrent.trackerInbox = make(chan TrackerResult)
	torrent.Download = 0
	torrent.Upload = 0
	torrent.Left = int64(len(file.Pieces) / 20)
	torrent.Picker = NewPiecePicker(&torrent, 16*1024)

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
		case p := <-t.newPeer:
			{
				t.PeerList = append(t.PeerList, p)
				p.start()
				p.SendBitfield(t.Picker.GetBitfield())
				if t.Picker.calculateInterested(p) {
					p.SetInterested(true)
				}
			}
		case p := <-t.remPeer:
			{
				for i, val := range t.PeerList {
					if val == p {
						t.PeerList = append(t.PeerList[:i], t.PeerList[i+1:]...)
					}
				}
				t.Picker.DecRefBitfield(p.pieces)
			}
		case mess := <-t.peerInbox:
			{
				go t.handleIncomingMessage(mess)
			}
		case ready := <-t.trackerReady:
			{
				t.announceToTracker(ready, TrackerNone)
			}
		case res := <-t.trackerInbox:
			{
				if res.Err != nil {
				} else if res.Val.Failure != nil {
				} else {
					fmt.Printf("next announce in -> %v\n", res.Val.Interval/60)
					go t.dialPeers(res.Val.PeerList)
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

func (t *Torrent) RemovePeer(p *peerConnection) {
	t.remPeer <- p
}

func (t *Torrent) AddPeer(p *peerConnection) {
	t.newPeer <- p
}

func (t *Torrent) ReceiveMessage(mess peerMessage) {
	t.peerInbox <- mess
}

func (t *Torrent) SignalTrackerReady(tracker *Tracker) {
	t.trackerReady <- tracker
}

func (t *Torrent) ReceiveTracker(res TrackerResult) {
	t.trackerInbox <- res
}

func (t *Torrent) handleIncomingMessage(mess peerMessage) {
	switch mess.Kind {
	case Have:
		{
			idx := binary.LittleEndian.Uint32(mess.Payload)
			t.Picker.IncRef(idx)
		}
	case Bitfield:
		{
			bf := utils.NewBitfield(t.Picker.pieceCount)
			bf.SetBitfield(mess.Payload)
			ok := t.Picker.IncRefBitfield(bf)
			if !ok {
				mess.Peer.cancel(Peer_invalid_bitfield)
				break
			}
		}
	}
}

func (t *Torrent) dialPeers(peers []Peer) {
	for _, p := range peers {
		go t.dialPeer(p)
	}
}

func (t *Torrent) dialPeer(p Peer) {
	conn, err := newPeerConnection(t, p, t.Ses.PeerID, 5)
	if err != nil {
		fmt.Printf("CONNECTION FAILED -> %v\n", err.Error())
		return
	}
	fmt.Printf("CONNECTION SUCCESS -> %v\n", string(conn.Info.Pid.String()))
	t.newPeer <- conn
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
