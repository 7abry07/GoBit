package protocol

import (
	"io"
	"net/http"
)

type result struct {
	Err error
	Val TrackerResponse
}

type TrackerManager struct {
	results chan<- result
}

func NewTrackerManager() (*TrackerManager, <-chan result) {
	ch := make(chan result)
	m := TrackerManager{}
	m.results = ch
	return &m, ch
}

func (m *TrackerManager) Send(req TrackerRequest) {
	switch req.Url.Scheme {
	case "http":
		{
			trackerUrl, err := req.EncodeHttp()
			if err != nil {
				m.results <- result{err, TrackerResponse{}}
				return
			}

			httpResp, err := http.Get(trackerUrl.String())
			if err != nil {
				m.results <- result{err, TrackerResponse{}}
				return
			}

			content, err := io.ReadAll(httpResp.Body)
			if err != nil {
				m.results <- result{err, TrackerResponse{}}
				return
			}

			resp, err := ParseTrackerResponseHttp(content, req)
			if err != nil {
				m.results <- result{err, TrackerResponse{}}
				return
			}

			m.results <- result{nil, resp}
		}
	case "udp":
		{
			// TODO
			m.results <- result{Tracker_invalid_scheme_err, TrackerResponse{}}
		}
	default:
		{
			m.results <- result{Tracker_invalid_scheme_err, TrackerResponse{}}
		}
	}
}
