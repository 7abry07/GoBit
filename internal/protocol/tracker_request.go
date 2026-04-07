package protocol

import (
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
)

type TrackerEventType int
type TrackerRequestKind int

const (
	TRACKER_NONE TrackerEventType = iota
	TRACKER_COMPLETED
	TRACKER_STARTED
	TRACKER_STOPPED
)

const (
	TrackerAnnounce TrackerRequestKind = iota
	TrackerScrape
)

type TrackerRequest struct {
	Infohash   [20]byte
	PeerID     [20]byte
	Downloaded int64
	Uploaded   int64
	Left       int64
	Numwant    uint32
	Ip         netip.Addr
	Port       uint16
	Key        uint32
	NoPID      uint8
	Compact    uint8
	Event      TrackerEventType
	Kind       TrackerRequestKind
}

func (req TrackerRequest) SerializeHttp(t HttpTracker) url.URL {
	fullUrl := url.URL{}
	if req.Kind == TrackerAnnounce {
		fullUrl = t.announce
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
	} else {
		fullUrl = t.scrape
		fullUrl.RawQuery = fmt.Sprintf("info_hash=%v", url.QueryEscape(string(req.Infohash[:])))
	}
	return fullUrl
}
