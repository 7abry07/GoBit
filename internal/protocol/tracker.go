package protocol

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Tracker struct {
	TrackerID  string
	Announce   url.URL
	Scrape     url.URL
	FailureCnt int
}

func NewTracker(announce url.URL) (*Tracker, error) {
	t := Tracker{}

	if announce.Scheme != "http" {
		return nil, Tracker_invalid_scheme_err
	}

	t.Announce = announce

	scrapeUrlPath := strings.Replace(announce.Path, "announce", "scrape", 1)
	if scrapeUrlPath == announce.Path {
		t.Scrape = url.URL{}
	} else {
		announce.Path = scrapeUrlPath
		t.Scrape = announce
	}
	t.TrackerID = ""

	return &t, nil
}

func (t *Tracker) SendAnnounce(event TrackerEventType, torrent *Torrent, clientPid PeerID, port uint16) (TrackerResponse, error) {
	req := TrackerRequest{}
	req.Compact = 1
	req.Downloaded = torrent.Download
	req.Uploaded = torrent.Upload
	req.Event = event
	req.Infohash = torrent.Info.InfoHash
	req.Kind = TrackerAnnounce
	req.Left = torrent.Left
	req.NoPID = 1
	req.Numwant = 200
	req.PeerID = clientPid
	req.Port = port

	res, err := t.send(req)
	if err != nil {
		return TrackerResponse{}, errors.New("http/deserialization error in tracker response")
	}
	if res.Failure != nil {
		return TrackerResponse{}, fmt.Errorf("tracker failure string -> %v", *res.Failure)
	}
	return res, nil
}

func (t *Tracker) send(req TrackerRequest) (TrackerResponse, error) {
	switch t.Announce.Scheme {
	case "http":
		{
			fullUrl := req.SerializeHttp(*t)
			httpResp, err := http.Get(fullUrl.String())
			if err != nil {
				return TrackerResponse{}, err
			}
			content, err := io.ReadAll(httpResp.Body)
			if err != nil {
				return TrackerResponse{}, err
			}
			resp, err := DeserializeTrackerResponseHttp(content, req)
			if err != nil {
				return TrackerResponse{}, err
			}
			if resp.trackerID != nil {
				t.TrackerID = *resp.trackerID
			}

			return resp, nil
		}
	case "udp":
		return TrackerResponse{}, Tracker_invalid_scheme_err
	default:
		panic(Tracker_invalid_scheme_err)
	}
}
