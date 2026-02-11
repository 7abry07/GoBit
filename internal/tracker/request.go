package tracker

import (
	"net/netip"
	"net/url"
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
