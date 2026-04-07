package protocol

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type HttpTracker struct {
	torrent *Torrent

	announce url.URL
	scrape   url.URL

	failureCnt int
}

func NewHttpTracker(torrent *Torrent, announce url.URL) (*HttpTracker, error) {
	t := HttpTracker{}

	if announce.Scheme != "http" {
		return nil, Tracker_invalid_scheme_err
	}

	t.torrent = torrent
	t.announce = announce

	scrapeUrlPath := strings.Replace(announce.Path, "announce", "scrape", 1)
	if scrapeUrlPath == announce.Path {
		t.scrape = url.URL{}
	} else {
		announce.Path = scrapeUrlPath
		t.scrape = announce
	}

	return &t, nil
}

func (t *HttpTracker) Announce(event TrackerEventType) {
	req := TrackerRequest{}
	req.Compact = 1
	req.Downloaded = t.torrent.Downloaded
	req.Uploaded = t.torrent.Uploaded
	req.Event = event
	req.Infohash = t.torrent.Info.InfoHash
	req.Kind = TrackerAnnounce
	req.Left = t.torrent.Left
	req.NoPID = 1
	req.Numwant = 200
	req.PeerID = t.torrent.Ses.PeerID
	req.Port = t.torrent.Ses.Port

	go t.send(req)
}

func (t *HttpTracker) Scrape() {
	// TODO
}

func (t *HttpTracker) Failure() {
	t.failureCnt++
}

func (t *HttpTracker) FailedCount() int {
	return t.failureCnt
}

func (t *HttpTracker) GetHost() string {
	return t.announce.Host
}

func (t *HttpTracker) send(req TrackerRequest) {
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
