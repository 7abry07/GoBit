package protocol

import (
	"GoBit/internal/utils"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/url"
	"time"
)

type UdpTrackerAction uint8

const (
	CONNECT UdpTrackerAction = iota
	ANNOUNCE
	SCRAPE
	ERROR
)

type UdpTracker struct {
	lastConnIdRequested time.Time
	connectionId        uint64
	// transactionId       uint32

	addr *net.UDPAddr
	sock *net.UDPConn
	name string

	torrent    *Torrent
	failureCnt int
}

func NewUdpTracker(torrent *Torrent, url url.URL) (*UdpTracker, error) {
	t := UdpTracker{}
	t.lastConnIdRequested = time.Now()
	t.connectionId = 0
	// t.transactionId = 0
	t.name = url.Host
	t.torrent = torrent

	if url.Scheme != "udp" {
		return nil, Tracker_invalid_scheme_err
	}

	endpoint, err := net.ResolveUDPAddr("udp", url.Host)
	if err != nil {
		return nil, err
	}

	t.addr = endpoint
	t.sock, err = net.DialUDP("udp", nil, endpoint)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func (t *UdpTracker) Announce(event TrackerEventType) {
	req := TrackerAnnounceRequest{}
	req.Downloaded = t.torrent.Downloaded
	req.Uploaded = t.torrent.Uploaded
	req.Event = event
	req.Infohash = t.torrent.Info.InfoHash
	req.Left = t.torrent.Left
	req.Numwant = 200
	req.PeerID = t.torrent.Ses.PeerID
	req.Port = t.torrent.Ses.Port

	go t.sendAnnounce(req)
}

func (t *UdpTracker) Scrape() {
	req := TrackerScrapeRequest{}
	req.infohashes = append(req.infohashes, t.torrent.Info.InfoHash)

	go t.sendScrape(req)
}

func (t *UdpTracker) Failure() {
	t.failureCnt++
}

func (t *UdpTracker) FailedCount() int {
	return t.failureCnt
}

func (t *UdpTracker) GetHost() string {
	return t.name
}

func (t *UdpTracker) sendScrape(req TrackerScrapeRequest) {
	if t.connectionId == 0 || time.Since(t.lastConnIdRequested) > time.Minute {
		if !t.connect() {
			return
		}
	}

	transactionId := rand.Uint32()
	buf := req.SerializeUdp(*t, transactionId)
	err := utils.WriteFullUDP(t.sock, t.addr, buf)
	if err != nil {
		t.torrent.SignalEvent(TrackerScrapeFailed{t, err})
		return
	}

	retransmissions := float64(0)
	readBuf := make([]byte, 4096)
	var packetLen int
	for retransmissions < 8 {
		t.sock.SetDeadline(time.Now().Add(time.Second * time.Duration(15*math.Pow(2, retransmissions))))
		packetLen, _, err = t.sock.ReadFromUDP(readBuf)
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			retransmissions++
		} else {
			retransmissions = 0
			break
		}
	}

	if err != nil {
		t.torrent.SignalEvent(TrackerScrapeFailed{t, err})
		return
	}
	res := TrackerScrapeResponse{}
	err = res.DeserializeUdp(t, req, readBuf, packetLen, transactionId)
	if err != nil {
		t.torrent.SignalEvent(TrackerScrapeFailed{t, err})
		return
	}

	t.torrent.SignalEvent(TrackerScrapeSuccessful{t, res})
}

func (t *UdpTracker) sendAnnounce(req TrackerAnnounceRequest) {
	if t.connectionId == 0 || time.Since(t.lastConnIdRequested) > time.Minute {
		if !t.connect() {
			return
		}
	}
	transactionId := rand.Uint32()

	buf := req.SerializeUdp(*t, transactionId)
	err := utils.WriteFullUDP(t.sock, t.addr, buf)
	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
		return
	}

	retransmissions := float64(0)
	readBuf := make([]byte, 4096)
	var packetLen int
	for retransmissions < 8 {
		t.sock.SetDeadline(time.Now().Add(time.Second * time.Duration(15*math.Pow(2, retransmissions))))
		packetLen, _, err = t.sock.ReadFromUDP(readBuf)
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			retransmissions++
		} else {
			retransmissions = 0
			break
		}
	}

	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
		return
	}
	res := TrackerAnnounceResponse{}
	err = res.DeserializeUdp(t, readBuf, packetLen, transactionId)
	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
		return
	}

	t.torrent.SignalEvent(TrackerAnnounceSuccessful{t, res})
}

func (t *UdpTracker) connect() bool {
	buf := []byte{}
	transactionId := rand.Uint32()
	buf = binary.BigEndian.AppendUint64(buf, 0x41727101980)
	buf = binary.BigEndian.AppendUint32(buf, uint32(CONNECT))
	buf = binary.BigEndian.AppendUint32(buf, transactionId)

	err := utils.WriteFullUDP(t.sock, t.addr, buf)
	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
		return false
	}

	retransmissions := float64(0)
	readBuf := make([]byte, 4096)
	var packetLen int
	for retransmissions < 8 {
		t.sock.SetDeadline(time.Now().Add(time.Second * time.Duration(15*math.Pow(2, retransmissions))))
		packetLen, _, err = t.sock.ReadFromUDP(readBuf)
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			retransmissions++
		} else {
			retransmissions = 0
			break
		}
	}

	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
		return false
	}

	if packetLen < 16 {
		t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
		return false
	}

	action := binary.BigEndian.Uint32(readBuf[0:4])
	respTransactionId := binary.BigEndian.Uint32(readBuf[4:8])
	if transactionId != respTransactionId {
		t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
		return false
	}

	switch byte(action) {
	case byte(CONNECT):
		{
			t.connectionId = binary.BigEndian.Uint64(readBuf[8:16])
			t.lastConnIdRequested = time.Now()
			return true
		}
	case byte(ERROR):
		{
			err := fmt.Errorf("error in tracker response: %v", readBuf[8:packetLen])
			t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
			return false
		}
	default:
		t.torrent.SignalEvent(TrackerAnnounceFailed{t, Tracker_invalid_resp_err})
		return false
	}
}
