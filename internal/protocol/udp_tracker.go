package protocol

import (
	"GoBit/internal/utils"
	"encoding/binary"
	"sync"

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

	addr *net.UDPAddr
	sock *net.UDPConn
	name string

	pending sync.Map

	torrent    *Torrent
	failureCnt int
}

func NewUdpTracker(torrent *Torrent, url url.URL) (*UdpTracker, error) {
	t := UdpTracker{}
	t.lastConnIdRequested = time.Now()
	t.connectionId = 0
	t.name = url.Host
	t.torrent = torrent
	t.pending = sync.Map{}

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

	go t.readLoop()

	return &t, nil
}

func (t *UdpTracker) sendAnnounce(req TrackerAnnounceRequest) {
	if t.connectionId == 0 || time.Since(t.lastConnIdRequested) > time.Minute {
		if !t.connect() {
			return
		}
	}
	transactionId := rand.Uint32()

	buf := req.SerializeUdp(t, transactionId)
	err := utils.WriteFullUDP(t.sock, t.addr, buf)
	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounced{t, TrackerAnnounceResponse{}, err})
		return
	}
	ch := make(chan []byte, 1)
	t.pending.Store(transactionId, ch)

	go func() {
		retransmissions := float64(0)
		for {
			select {
			case readBuf := <-ch:
				{
					res := TrackerAnnounceResponse{}
					err = res.DeserializeUdp(t, readBuf)

					if err != nil {
						t.torrent.SignalEvent(TrackerAnnounced{t, TrackerAnnounceResponse{}, err})
						return
					}
					t.torrent.SignalEvent(TrackerAnnounced{t, res, nil})
					return
				}
			case <-time.After(time.Second * time.Duration(15*math.Pow(2, retransmissions))):
				{
					err := utils.WriteFullUDP(t.sock, t.addr, buf)
					if err != nil {
						t.torrent.SignalEvent(TrackerAnnounced{t, TrackerAnnounceResponse{}, err})
						return
					}

					if retransmissions == 8 {
						retransmissions = 0
						return
					}
					retransmissions++
				}
			}
		}
	}()
}

func (t *UdpTracker) connect() bool {
	buf := []byte{}
	transactionId := rand.Uint32()
	buf = binary.BigEndian.AppendUint64(buf, 0x41727101980)
	buf = binary.BigEndian.AppendUint32(buf, uint32(CONNECT))
	buf = binary.BigEndian.AppendUint32(buf, transactionId)

	err := utils.WriteFullUDP(t.sock, t.addr, buf)
	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounced{t, TrackerAnnounceResponse{}, err})
		return false
	}
	ch := make(chan []byte, 1)
	t.pending.Store(transactionId, ch)

	retransmissions := float64(0)
	for {
		select {
		case readBuf := <-ch:
			{
				action := binary.BigEndian.Uint32(readBuf[0:4])

				switch byte(action) {
				case byte(CONNECT):
					{
						t.connectionId = binary.BigEndian.Uint64(readBuf[8:16])
						t.lastConnIdRequested = time.Now()
						return true
					}
				case byte(ERROR):
					{
						err := fmt.Errorf("error in tracker response: %v", string(readBuf[8:]))
						t.torrent.SignalEvent(TrackerAnnounced{t, TrackerAnnounceResponse{}, err})
						return false
					}
				}
			}
		case <-time.After(time.Second * time.Duration(15*math.Pow(2, retransmissions))):
			{
				err := utils.WriteFullUDP(t.sock, t.addr, buf)
				if err != nil {
					t.torrent.SignalEvent(TrackerRemoved{t, err})
					return false
				}

				if retransmissions == 8 {
					retransmissions = 0
				}
				retransmissions++
			}
		}
	}
}

func (t *UdpTracker) sendScrape(req TrackerScrapeRequest) {
	if t.connectionId == 0 || time.Since(t.lastConnIdRequested) > time.Minute {
		if !t.connect() {
			return
		}
	}

	transactionId := rand.Uint32()
	buf := req.SerializeUdp(t, transactionId)
	err := utils.WriteFullUDP(t.sock, t.addr, buf)
	if err != nil {
		t.torrent.SignalEvent(TrackerScraped{t, TrackerScrapeResponse{}, err})
		return
	}
	ch := make(chan []byte, 1)
	t.pending.Store(transactionId, ch)

	go func() {
		retransmissions := float64(0)
		for {
			select {
			case readBuf := <-ch:
				{
					res := TrackerScrapeResponse{}
					err = res.DeserializeUdp(t, req, readBuf)
					if err != nil {
						t.torrent.SignalEvent(TrackerScraped{t, TrackerScrapeResponse{}, err})
						return
					}
					t.torrent.SignalEvent(TrackerScraped{t, res, nil})
					return
				}
			case <-time.After(time.Second * time.Duration(15*math.Pow(2, retransmissions))):
				{
					err := utils.WriteFullUDP(t.sock, t.addr, buf)
					if err != nil {
						t.torrent.SignalEvent(TrackerScraped{t, TrackerScrapeResponse{}, err})
						return
					}

					if retransmissions == 8 {
						retransmissions = 0
						return
					}
					retransmissions++
				}
			}
		}
	}()
}

func (t *UdpTracker) readLoop() {
	for {
		readBuf := make([]byte, 4096)
		packetLen, _, err := t.sock.ReadFromUDP(readBuf)
		if err != nil {
			t.torrent.SignalEvent(TrackerRemoved{t, err})
			break
		}

		if packetLen < 8 {
			continue
		}

		tsId := binary.BigEndian.Uint32(readBuf[4:8])
		ch, ok := t.pending.Load(tsId)
		if !ok {
			continue
		}

		ch.(chan []byte) <- readBuf[:packetLen]
		t.pending.Delete(tsId)
	}

	for key, val := range t.pending.Range {
		close(val.(chan []byte))
		t.pending.Delete(key.(uint32))
	}
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

func (t *UdpTracker) Stop() {
	t.sock.Close()
}

func (t *UdpTracker) Failure() {
	t.failureCnt++
}

func (t *UdpTracker) ResetFailure() {
	t.failureCnt = 0
}

func (t *UdpTracker) FailedCount() int {
	return t.failureCnt
}

func (t *UdpTracker) GetHost() string {
	return t.name
}
