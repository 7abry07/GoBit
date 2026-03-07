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
	cancel context.CancelFunc

	TrackerID string
	Announce  url.URL
	Scrape    url.URL
	Tier      uint8

	Out chan TrackerRequest

	intervalTimer    *time.Timer
	previousInterval int
	torr             *Torrent
}

func NewTracker(announce url.URL, tier uint8, torrent *Torrent) (*Tracker, error) {
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
	t.Out = make(chan TrackerRequest)
	t.torr = torrent
	t.Tier = tier
	t.previousInterval = 1800
	t.ctx, t.cancel = context.WithCancel(torrent.ctx)

	t.intervalTimer = time.NewTimer(time.Hour)
	if !t.intervalTimer.Stop() {
		<-t.intervalTimer.C
	}

	go t.loop()

	return &t, nil
}

func (t *Tracker) loop() {
	for {
		select {
		case <-t.ctx.Done():
			{
				fmt.Printf("TRACKER REMOVED -> %v\n", t.Announce.String())
				t.intervalTimer.Stop()
				return
			}
		case req := <-t.Out:
			{
				go t.send(req)
			}
		case _ = <-t.intervalTimer.C:
			{
				t.torr.SignalTrackerReady(t)
			}
		}
	}
}

func (t *Tracker) send(req TrackerRequest) {
	defaultResp := TrackerResponse{}
	defaultResp.Tracker_ = t
	switch t.Announce.Scheme {
	case "http":
		{
			fullUrl := req.SerializeHttp(*t)
			httpResp, err := http.Get(fullUrl.String())
			if err != nil {
				t.torr.ReceiveTracker(TrackerResult{err, defaultResp})
				go t.schedule(t.previousInterval)
				return
			}
			content, err := io.ReadAll(httpResp.Body)
			if err != nil {
				t.torr.ReceiveTracker(TrackerResult{err, defaultResp})
				go t.schedule(t.previousInterval)
				return
			}
			resp, err := DeserializeTrackerResponseHttp(content, req)
			if err != nil {
				t.torr.ReceiveTracker(TrackerResult{err, defaultResp})
				go t.schedule(t.previousInterval)
				return
			}
			if resp.trackerID != nil {
				t.TrackerID = *resp.trackerID
			}

			t.torr.ReceiveTracker(TrackerResult{nil, resp})
			go t.schedule(int(resp.Interval))
			t.previousInterval = int(resp.Interval)
		}
	case "udp":
		t.torr.ReceiveTracker(TrackerResult{Tracker_invalid_scheme_err, TrackerResponse{}})
	default:
		panic(Tracker_invalid_scheme_err)
	}
}

func (t *Tracker) schedule(seconds int) {
	t.intervalTimer.Reset(time.Second * time.Duration(seconds))
}
