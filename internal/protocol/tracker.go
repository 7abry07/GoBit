package protocol

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Tracker struct {
	torrent *Torrent

	Announce url.URL
	Scrape   url.URL

	FailureCnt int
}

func NewTracker(torrent *Torrent, announce url.URL) (*Tracker, error) {
	t := Tracker{}

	if announce.Scheme != "http" {
		return nil, Tracker_invalid_scheme_err
	}

	t.torrent = torrent
	t.Announce = announce

	scrapeUrlPath := strings.Replace(announce.Path, "announce", "scrape", 1)
	if scrapeUrlPath == announce.Path {
		t.Scrape = url.URL{}
	} else {
		announce.Path = scrapeUrlPath
		t.Scrape = announce
	}

	return &t, nil
}

func (t *Tracker) SendAnnounce(ih [20]byte, d, u, l int64, event TrackerEventType, clientPid PeerID, port uint16) {
	req := TrackerRequest{}
	req.Compact = 1
	req.Downloaded = d
	req.Uploaded = u
	req.Event = event
	req.Infohash = ih
	req.Kind = TrackerAnnounce
	req.Left = l
	req.NoPID = 1
	req.Numwant = 200
	req.PeerID = clientPid
	req.Port = port

	go t.send(req)
}

func (t *Tracker) SendScrape(ih [20]byte, d, u, l int64, event TrackerEventType, clientPid PeerID, port uint16) {
}

func (t *Tracker) send(req TrackerRequest) {
	switch t.Announce.Scheme {
	case "http":
		{
			fullUrl := req.SerializeHttp(*t)
			httpResp, err := http.Get(fullUrl.String())
			if err != nil {
				t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
			}
			content, err := io.ReadAll(httpResp.Body)
			if err != nil {
				t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
			}
			resp, err := DeserializeTrackerResponseHttp(content, req)
			if err != nil {
				t.torrent.SignalEvent(TrackerAnnounceFailed{t, err})
			}
			if resp.Failure != nil {
				t.torrent.SignalEvent(TrackerAnnounceFailed{t, fmt.Errorf("tracker failure string -> %v", *resp.Failure)})
			}
			t.torrent.SignalEvent(TrackerAnnounceSuccessful{t, resp})
		}
	case "udp":
		panic(Tracker_invalid_scheme_err)
	default:
		panic(Tracker_invalid_scheme_err)
	}
}
