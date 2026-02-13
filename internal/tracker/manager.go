package tracker

import (
	"io"
	"net/http"
)

type result struct {
	Err error
	Val Response
}

type manager struct {
	results chan<- result
}

func NewManager() (*manager, <-chan result) {
	ch := make(chan result)
	m := manager{}
	m.results = ch
	return &m, ch
}

func (m *manager) Send(req Request) {
	switch req.Url.Scheme {
	case "http":
		{
			trackerUrl, err := req.EncodeHttp()
			if err != nil {
				m.results <- result{err, Response{}}
				return
			}

			httpResp, err := http.Get(trackerUrl.String())
			if err != nil {
				m.results <- result{err, Response{}}
				return
			}

			content, err := io.ReadAll(httpResp.Body)
			if err != nil {
				m.results <- result{err, Response{}}
				return
			}

			resp, err := ParseHttp(content, req)
			if err != nil {
				m.results <- result{err, Response{}}
				return
			}

			m.results <- result{nil, resp}
		}
	case "udp":
		{
			// TODO
			m.results <- result{invalid_scheme_err, Response{}}
		}
	default:
		{
			m.results <- result{invalid_scheme_err, Response{}}
		}
	}
}
