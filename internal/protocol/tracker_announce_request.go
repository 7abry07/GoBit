package protocol

import (
	"encoding/binary"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
)

type TrackerAnnounceRequest struct {
	Infohash   [20]byte
	PeerID     [20]byte
	Downloaded uint64
	Uploaded   uint64
	Left       uint64
	Numwant    uint32
	Ip         netip.Addr
	Port       uint16
	Key        uint32
	NoPID      uint8
	Compact    uint8
	Event      TrackerEventType
}

func (req TrackerAnnounceRequest) SerializeHttp(t HttpTracker) url.URL {
	fullUrl := t.announce
	eventStr := []string{"none", "completed", "started", "stopped"}
	query := fmt.Sprintf(
		"info_hash=%v"+
			"&peer_id=%v"+
			"&port=%v"+
			"&uploaded=%v"+
			"&downloaded=%v"+
			"&left=%v"+
			"&compact=%v"+
			"&no_peer_id=%v"+
			"&event=%v"+
			"&numwant=%v"+
			"&ip=%v"+
			"&key=%v"+
			"&trackerid=%v",
		url.QueryEscape(string(req.Infohash[:])),
		url.QueryEscape(string(req.PeerID[:])),
		url.QueryEscape(strconv.Itoa(int(req.Port))),
		url.QueryEscape(strconv.Itoa(int(req.Uploaded))),
		url.QueryEscape(strconv.Itoa(int(req.Downloaded))),
		url.QueryEscape(strconv.Itoa(int(req.Left))),
		url.QueryEscape(strconv.Itoa(int(req.Compact))),
		url.QueryEscape(strconv.Itoa(int(req.NoPID))),
		url.QueryEscape(eventStr[req.Event]),
		url.QueryEscape(strconv.Itoa(int(req.Numwant))),
		url.QueryEscape(req.Ip.String()),
		url.QueryEscape(strconv.Itoa(int(req.Key))),
		"",
	)
	fullUrl.RawQuery = query

	return fullUrl
}

func (req TrackerAnnounceRequest) SerializeUdp(t UdpTracker, transactionId uint32) []byte {
	buf := []byte{}
	buf = binary.BigEndian.AppendUint64(buf, t.connectionId)
	buf = binary.BigEndian.AppendUint32(buf, uint32(ANNOUNCE))
	buf = binary.BigEndian.AppendUint32(buf, transactionId)
	buf = append(buf, t.torrent.Info.InfoHash[:]...)
	buf = append(buf, t.torrent.Ses.PeerID[:]...)
	buf = binary.BigEndian.AppendUint64(buf, t.torrent.Downloaded)
	buf = binary.BigEndian.AppendUint64(buf, t.torrent.Left)
	buf = binary.BigEndian.AppendUint64(buf, t.torrent.Uploaded)
	buf = binary.BigEndian.AppendUint32(buf, uint32(req.Event))
	buf = binary.BigEndian.AppendUint32(buf, 0)
	buf = binary.BigEndian.AppendUint32(buf, 0)
	buf = binary.BigEndian.AppendUint32(buf, 200)
	buf = binary.BigEndian.AppendUint16(buf, t.torrent.Ses.Port)

	return buf
}
