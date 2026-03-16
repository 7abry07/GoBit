package protocol

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	t.clientPid = session.PeerID
	t.port = session.Port
	t.ctx, t.cancel = context.WithCancelCause(torrent.ctx)

	go t.loop()

	return &t, nil
}

func (t *Tracker) loop() {
	select {
	case <-t.ctx.Done():
		{
			go t.SendAnnounce(TrackerStopped)
			t.torr.RemoveTracker(t)
			return
		}
	}
}

func (t *Tracker) SendAnnounce(event TrackerEventType) (TrackerResponse, bool) {
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
		return TrackerResponse{}, false
	}
	if res.Failure != nil {
		t.cancel(fmt.Errorf("announce failed (%v)", *res.Failure))
		return TrackerResponse{}, false
	}
	return res, true
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
