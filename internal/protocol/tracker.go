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

type TrackerResult struct {
	Err error
	Val TrackerResponse
}

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

	res := t.send(req)
	if res.Err != nil {
		t.cancel(res.Err)
		return time.Now(), false
	}
	if res.Val.Failure != nil {
		err := fmt.Errorf("announce failed (%v)", *res.Val.Failure)
		t.cancel(err)
		return time.Now(), false
	}

	//
	fmt.Printf("[%v] SENDING PEERS (reannounce in %v)\n", t.Announce.String(), (time.Second * time.Duration(res.Val.Interval)))
	//

	for _, e := range res.Val.PeerList {
		go t.torr.AddPeer(e)
	}

	return time.Now().Add(time.Second * time.Duration(res.Val.Interval)), true
}

func (t *Tracker) send(req TrackerRequest) TrackerResult {
	defaultResp := TrackerResponse{}
	defaultResp.Tracker_ = t
	switch t.Announce.Scheme {
	case "http":
		{
			fullUrl := req.SerializeHttp(*t)
			httpResp, err := http.Get(fullUrl.String())
			if err != nil {
				return TrackerResult{err, defaultResp}
			}
			content, err := io.ReadAll(httpResp.Body)
			if err != nil {
				return TrackerResult{err, defaultResp}
			}
			resp, err := DeserializeTrackerResponseHttp(content, req)
			if err != nil {
				return TrackerResult{err, defaultResp}
			}
			if resp.trackerID != nil {
				t.TrackerID = *resp.trackerID
			}

			return TrackerResult{nil, resp}
		}
	case "udp":
		return TrackerResult{Tracker_invalid_scheme_err, defaultResp}
	default:
		panic(Tracker_invalid_scheme_err)
	}
}
