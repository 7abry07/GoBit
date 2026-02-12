package tracker

import (
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

type eventType int
type requestKind int

const (
	None eventType = iota
	Completed
	Started
	Stopped
)

const (
	Announce requestKind = iota
	Scrape
)

type Request struct {
	Url        url.URL
	TrackerID  string
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
	Event      eventType
	Kind       requestKind
}

func (r Request) EncodeHttp() (url.URL, error) {
	trackerUrl := r.Url
	if r.Kind == Announce {
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
				"numwant=%v"+
				"&ip=%v"+
				"&key=%v"+
				"&trackerid=%v",
			url.QueryEscape(string(r.Infohash[:])),
			url.QueryEscape(string(r.PeerID[:])),
			url.QueryEscape(strconv.Itoa(int(r.Port))),
			url.QueryEscape(strconv.Itoa(int(r.Uploaded))),
			url.QueryEscape(strconv.Itoa(int(r.Downloaded))),
			url.QueryEscape(strconv.Itoa(int(r.Left))),
			url.QueryEscape(strconv.Itoa(int(r.Compact))),
			url.QueryEscape(strconv.Itoa(int(r.NoPID))),
			url.QueryEscape(eventStr[r.Event]),
			url.QueryEscape(strconv.Itoa(int(r.Numwant))),
			url.QueryEscape(r.Ip.String()),
			url.QueryEscape(strconv.Itoa(int(r.Key))),
			url.QueryEscape(r.TrackerID),
		)
		trackerUrl.RawQuery = query
	} else {
		path := strings.Replace(trackerUrl.Path, "announce", "scrape", 1)
		if path == trackerUrl.Path {
			return url.URL{}, scrape_not_supported_err
		}
		trackerUrl.Path = path
		trackerUrl.RawQuery = fmt.Sprintf("info_hash=%v", url.QueryEscape(string(r.Infohash[:])))
	}
	return trackerUrl, nil
}
