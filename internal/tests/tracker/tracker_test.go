package tracker_test

import (
	"GoBit/internal/tracker"
	"testing"
)

func TestHttpAnnounceResponse(t *testing.T) {
	req := tracker.Request{}
	resp := "d8:completei0e10:downloadedi0e10:incompletei0e8:intervali0e12:min intervali0e5:peers6:\xff\xff\xff\xff\xff\xffe"

	req.Kind = tracker.Announce

	parsed, err := tracker.ParseHttp([]byte(resp), req)
	if err != nil {
		t.Errorf("unexpected error -> %v", err.Error())
	}

	if parsed.PeerList[0].IpPort.String() != "255.255.255.255:65535" {
		t.Errorf("peer expected: <%v>[%v] | got: <%v>[%v]", "255.255.255.255", 65535, parsed.PeerList[0].IpPort.Addr(), parsed.PeerList[0].IpPort.Port())
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
	req := tracker.Request{}
	resp := "d5:files" +
		"d20:\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00" +
		"d8:completei0e" +
		"10:downloadedi0e" +
		"10:incompletei0eeee"

	req.Kind = tracker.Scrape
	req.Infohash = [20]byte{0}

	parsed, err := tracker.ParseHttp([]byte(resp), req)
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
	req := tracker.Request{}
	resp := "d8:completei0e" +
		"10:downloadedi0e" +
		"10:incompletei0e" +
		"8:intervali0e" +
		"12:min intervali0e" +
		"5:peersld2:ip15:255.255.255.255" +
		"7:peer id20:\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00" +
		"4:porti65535eeee"

	req.Kind = tracker.Announce

	parsed, err := tracker.ParseHttp([]byte(resp), req)
	if err != nil {
		t.Errorf("unexpected error -> %v", err.Error())
	}

	if parsed.PeerList[0].IpPort.String() != "255.255.255.255:65535" {
		t.Errorf("'peer ip:port' expected: <%v>[%v] | got: <%v>[%v]", "255.255.255.255", 65535, parsed.PeerList[0].IpPort.Addr(), parsed.PeerList[0].IpPort.Port())
	}

	if parsed.PeerList[0].PeerID != [20]byte{0} {
		t.Errorf("'peer id' expected: %v | got: %x", "00000000000000000000", parsed.PeerList[0].PeerID)
	}
}
