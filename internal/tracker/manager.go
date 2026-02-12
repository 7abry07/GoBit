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
			resp, err := m.sendHttp(req)
			if err != nil {
				m.results <- result{err, Response{}}
				return
			}
			m.results <- result{nil, resp}
			return
		}
	case "udp":
		{
			// TODO
		}
		fallthrough
	default:
		{
			m.results <- result{invalid_scheme_err, Response{}}
		}
	}
}

func (m *manager) sendHttp(req Request) (Response, error) {
	trackerUrl, err := req.EncodeHttp()
	if err != nil {
		return Response{}, err
	}

	httpResp, err := http.Get(trackerUrl.String())
	if err != nil {
		return Response{}, err
	}

	content, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, err
	}

	resp, err := ParseHttp(content, req)
	if err != nil {
		return Response{}, err
	}
	return resp, nil
}
