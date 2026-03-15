package protocol

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Tracker struct {
	ctx    context.Context
	cancel context.CancelCauseFunc

	TrackerID string
	Announce  url.URL
	Scrape    url.URL

	torr *Torrent

	clientPid PeerID
	port      uint16
}

func NewTracker(announce url.URL, torrent *Torrent, session *Session) (*Tracker, error) {
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
	t.torr = torrent
	t.ctx, t.cancel = context.WithCancelCause(torrent.ctx)
	t.clientPid = session.PeerID
	t.port = session.Port

	return &t, nil
}

func (t *Tracker) SendAnnounce(event TrackerEventType) (time.Time, bool) {
	req := TrackerRequest{}
	req.Compact = 1
	req.Downloaded = t.torr.Download
	req.Uploaded = t.torr.Upload
	req.Event = event
	req.Infohash = t.torr.Info.InfoHash
	req.Kind = TrackerAnnounce
	req.Left = t.torr.Left
	req.NoPID = 1
	req.Numwant = 200
	req.PeerID = t.clientPid
	req.Port = t.port

	res, err := t.send(req)
	if err != nil {
		t.cancel(err)
		return time.Now(), false
	}
	if res.Failure != nil {
		err := fmt.Errorf("announce failed (%v)", res.Failure)
		t.cancel(err)
		return time.Now(), false
	}

	fmt.Printf("[%v] -> SENDING PEERS (reannounce in %v)\n", t.Announce.String(), (time.Second * time.Duration(res.Interval)))

	for _, entry := range res.PeerList {
		peer := NewPeer(entry.IpPort)
		go t.torr.AddPeer(peer)
	}

	return time.Now().Add(time.Second * time.Duration(res.Interval)), true
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
