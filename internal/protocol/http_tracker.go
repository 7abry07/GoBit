package protocol

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HttpTracker struct {
	torrent *Torrent

	announce url.URL
	scrape   url.URL

	failureCnt int
}

func NewHttpTracker(torrent *Torrent, announce url.URL) (*HttpTracker, error) {
	t := HttpTracker{}

	if announce.Scheme != "http" && announce.Scheme != "https" {
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
	req := TrackerAnnounceRequest{}
	req.Compact = 1
	req.Downloaded = t.torrent.Downloaded
	req.Uploaded = t.torrent.Uploaded
	req.Event = event
	req.Infohash = t.torrent.Info.InfoHash
	req.Left = t.torrent.Left
	req.NoPID = 1
	req.Numwant = 200
	req.PeerID = t.torrent.Ses.PeerID
	req.Port = t.torrent.Ses.Port

	go t.sendAnnounce(req)
}

func (t *HttpTracker) Scrape() {
	req := TrackerScrapeRequest{}
	req.infohashes = append(req.infohashes, t.torrent.Info.InfoHash)

	go t.sendScrape(req)
}

func (t *HttpTracker) Stop() {}

func (t *HttpTracker) Failure() {
	t.failureCnt++
}

func (t *HttpTracker) ResetFailure() {
	t.failureCnt = 0
}

func (t *HttpTracker) FailedCount() int {
	return t.failureCnt
}

func (t *HttpTracker) GetHost() string {
	return t.announce.Host
}

func (t *HttpTracker) sendScrape(req TrackerScrapeRequest) {
	fullUrl := req.SerializeHttp(t)

	var err error
	var httpResp *http.Response
	retransmissions := float64(0)

	for retransmissions < 8 {
		client := http.Client{Timeout: time.Second * time.Duration(15*math.Pow(2, retransmissions))}
		httpResp, err = client.Get(fullUrl.String())
		if errors.Is(err, context.DeadlineExceeded) {
			retransmissions++
		} else {
			retransmissions = 0
			break
		}
	}

	if err != nil {
		t.torrent.SignalEvent(TrackerScraped{t, TrackerScrapeResponse{}, err})
		return
	}
	content, err := io.ReadAll(httpResp.Body)
	if err != nil {
		t.torrent.SignalEvent(TrackerScraped{t, TrackerScrapeResponse{}, err})
		return
	}
	res := TrackerScrapeResponse{}
	err = res.DeserializeHttp(t, content, req)
	if err != nil {
		t.torrent.SignalEvent(TrackerScraped{t, TrackerScrapeResponse{}, err})
		return
	}

	t.torrent.SignalEvent(TrackerScraped{t, res, nil})
}

func (t *HttpTracker) sendAnnounce(req TrackerAnnounceRequest) {
	fullUrl := req.SerializeHttp(t)

	var err error
	var httpResp *http.Response
	retransmissions := float64(0)

	for retransmissions < 8 {
		client := http.Client{Timeout: time.Second * time.Duration(15*math.Pow(2, retransmissions))}
		httpResp, err = client.Get(fullUrl.String())
		if errors.Is(err, context.DeadlineExceeded) {
			retransmissions++
		} else {
			retransmissions = 0
			break
		}
	}

	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounced{t, TrackerAnnounceResponse{}, err})
		return
	}
	content, err := io.ReadAll(httpResp.Body)
	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounced{t, TrackerAnnounceResponse{}, err})
		return
	}
	res := TrackerAnnounceResponse{}
	err = res.DeserializeHttp(content)
	if err != nil {
		t.torrent.SignalEvent(TrackerAnnounced{t, TrackerAnnounceResponse{}, err})
		return
	}
	if res.Failure != nil {
		t.torrent.SignalEvent(TrackerAnnounced{t, TrackerAnnounceResponse{}, fmt.Errorf("tracker failure string -> %v", *res.Failure)})
		return
	}
	t.torrent.SignalEvent(TrackerAnnounced{t, res, nil})
}
