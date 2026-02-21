package protocol_test

import (
	"GoBit/internal/protocol"
	"net/netip"
	"net/url"
	"testing"
)

func TestHttpAnnounceRequest(t *testing.T) {
	tracker := protocol.Tracker{}
	req := protocol.TrackerRequest{}

	req.Downloaded = 1
	req.Uploaded = 1
	req.Event = protocol.TrackerNone
	req.Infohash =
		[20]byte{
			0xde, 0x2f, 0xee, 0x7c, 0xd8,
			0xf3, 0x25, 0x14, 0xdc, 0x13,
			0x8b, 0x4c, 0xdd, 0x53, 0xc9,
			0x3d, 0x7d, 0x7a, 0x1e, 0xb6,
		}

	req.Key = 0
	req.NoPID = 1
	req.PeerID =
		[20]byte{
			0x7a, 0x1c, 0xe4, 0x92, 0x3f,
			0xb8, 0x0d, 0x6e, 0x55, 0xa3,
			0xdf, 0x21, 0x9b, 0x44, 0x78,
			0xcc, 0x02, 0xf1, 0x6d, 0x90,
		}
	tracker.TrackerID = "hello"
	req.Compact = 1
	req.Ip, _ = netip.ParseAddr("255.255.255.255")
	u, _ := url.Parse("http://hello:7777/announce")
	tracker.Announce = *u
	req.Kind = protocol.TrackerAnnounce
	req.Port = 6881
	req.Left = 4000

	fullUrl, err := req.SerializeHttp(tracker)
	if err != nil {
		t.Errorf("unexpected error -> %v", err.Error())
	}

	if fullUrl.Scheme != "http" {
		t.Errorf("'scheme' expected: %v | got: %v", "http", fullUrl.Scheme)
	}

	if fullUrl.Host != "hello:7777" {
		t.Errorf("'host' expected: %v | got: %v", "hello:7777", fullUrl.Host)
	}

	if fullUrl.Path != "/announce" {
		t.Errorf("'path' expected: %v | got: %v", "/announce", fullUrl.Path)
	}

	if fullUrl.Query().Get("info_hash") != "\xde\x2f\xee\x7c\xd8\xf3\x25\x14\xdc\x13\x8b\x4c\xdd\x53\xc9\x3d\x7d\x7a\x1e\xb6" {
		t.Errorf("'info_hash' expected: %v | got: %v", "\xde\x2f\xee\x7c\xd8\xf3\x25\x14\xdc\x13\x8b\x4c\xdd\x53\xc9\x3d\x7d\x7a\x1e\xb6", fullUrl.Query().Get("info_hash"))
	}
}

func TestHttpAnnounceResponse(t *testing.T) {
	req := protocol.TrackerRequest{}
	resp := "d8:completei0e" +
		"10:downloadedi0e" +
		"10:incompletei0e" +
		"8:intervali0e" +
		"12:min intervali0e" +
		"5:peers6:\xff\xff\xff\xff\xff\xff" +
		"6:peers618:\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xffe"

	req.Kind = protocol.TrackerAnnounce

	parsed, err := protocol.DeserializeTrackerResponseHttp([]byte(resp), req)
	if err != nil {
		t.Errorf("unexpected error -> %v", err.Error())
	}

	if parsed.PeerList[0].IpPort.String() != "255.255.255.255:65535" {
		t.Errorf("peer expected: <%v>[%v] | got: <%v>[%v]", "255.255.255.255", 65535, parsed.PeerList[0].IpPort.Addr(), parsed.PeerList[0].IpPort.Port())
	}

	if parsed.PeerList[1].IpPort.String() != "[ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff]:65535" {
		t.Errorf("peer expected: <%v>[%v] | got: <%v>[%v]", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff", 65535, parsed.PeerList[1].IpPort.Addr(), parsed.PeerList[1].IpPort.Port())
	}

	if parsed.Complete != 0 {
		t.Errorf("'complete' expected: %v | got: %v", 0, parsed.Complete)
	}

	if parsed.Incomplete != 0 {
		t.Errorf("'incomplete' expected: %v | got: %v", 0, parsed.Incomplete)
	}

	if parsed.Downloaded != 0 {
		t.Errorf("'downloaded' expected: %v | got: %v", 0, parsed.Downloaded)
	}

	if parsed.Interval != 0 {
		t.Errorf("'interval' expected: %v | got: %v", 0, parsed.Interval)
	}

	if parsed.MinInterval != 0 {
		t.Errorf("'min interval' expected: %v | got: %v", 0, parsed.MinInterval)
	}
}

func TestHttpScrapeResponse(t *testing.T) {
	req := protocol.TrackerRequest{}
	resp := "d5:files" +
		"d20:\xde\x2f\xee\x7c\xd8\xf3\x25\x14\xdc\x13\x8b\x4c\xdd\x53\xc9\x3d\x7d\x7a\x1e\xb6" +
		"d8:completei0e" +
		"10:downloadedi0e" +
		"10:incompletei0eeee"

	req.Kind = protocol.TrackerScrape
	req.Infohash =
		[20]byte{
			0xde, 0x2f, 0xee, 0x7c, 0xd8,
			0xf3, 0x25, 0x14, 0xdc, 0x13,
			0x8b, 0x4c, 0xdd, 0x53, 0xc9,
			0x3d, 0x7d, 0x7a, 0x1e, 0xb6,
		}

	parsed, err := protocol.DeserializeTrackerResponseHttp([]byte(resp), req)
	if err != nil {
		t.Errorf("unexpected error -> %v", err.Error())
	}

	if parsed.Complete != 0 {
		t.Errorf("'complete' expected: %v | got: %v", 0, parsed.Complete)
	}

	if parsed.Incomplete != 0 {
		t.Errorf("'incomplete' expected: %v | got: %v", 0, parsed.Incomplete)
	}

	if parsed.Downloaded != 0 {
		t.Errorf("'downloaded' expected: %v | got: %v", 0, parsed.Downloaded)
	}
}

func TestBencodedPeers(t *testing.T) {
	req := protocol.TrackerRequest{}
	resp := "d8:completei0e" +
		"10:downloadedi0e" +
		"10:incompletei0e" +
		"8:intervali0e" +
		"12:min intervali0e" +
		"5:peersld2:ip15:255.255.255.255" +
		"7:peer id20:\x7a\x1c\xe4\x92\x3f\xb8\x0d\x6e\x55\xa3\xdf\x21\x9b\x44\x78\xcc\x02\xf1\x6d\x90" +
		"4:porti65535eeee"

	req.Kind = protocol.TrackerAnnounce

	parsed, err := protocol.DeserializeTrackerResponseHttp([]byte(resp), req)
	if err != nil {
		t.Errorf("unexpected error -> %v", err.Error())
	}

	if parsed.PeerList[0].IpPort.String() != "255.255.255.255:65535" {
		t.Errorf("'peer ip:port' expected: <%v>[%v] | got: <%v>[%v]", "255.255.255.255", 65535, parsed.PeerList[0].IpPort.Addr(), parsed.PeerList[0].IpPort.Port())
	}

	if parsed.PeerList[0].PeerID != [20]byte{
		0x7a, 0x1c, 0xe4, 0x92, 0x3f,
		0xb8, 0x0d, 0x6e, 0x55, 0xa3,
		0xdf, 0x21, 0x9b, 0x44, 0x78,
		0xcc, 0x02, 0xf1, 0x6d, 0x90} {
		t.Errorf("'peer id' expected: %v | got: %x", "\x7a\x1c\xe4\x92\x3f\xb8\x0d\x6e\x55\xa3\xdf\x21\x9b\x44\x78\xcc\x02\xf1\x6d\x90", parsed.PeerList[0].PeerID)
	}
}
