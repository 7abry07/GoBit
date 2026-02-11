package tracker

import (
	"GoBit/internal/bencode"
	"GoBit/internal/torrent"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

type Result struct {
	Err  error
	Resp Response
}

type manager struct {
	httpUrls map[url.URL]string
	results  chan<- Result
}

func NewManager(results chan<- Result) *manager {
	m := manager{}
	m.results = results
	return &m
}

func (m *manager) Send(req Request) {
	switch req.Url.Scheme {
	case "http":
		{
			resp, err := m.sendHttp(req)
			if err != nil {
				m.results <- Result{err, Response{}}
				return
			}
			m.results <- Result{nil, resp}
			return
		}
	case "udp":
		{
			// TODO
		}
		fallthrough
	default:
		{
			m.results <- Result{invalid_scheme_err, Response{}}
		}
	}
}

func (m *manager) sendHttp(req Request) (Response, error) {
	trackerUrl := req.Url

	_, exists := m.httpUrls[req.Url]
	if exists {
		req.TrackerID = m.httpUrls[req.Url]
	}

	if req.Kind == Announce {
		eventStr := []string{"none", "completed", "started", "stopped"}
		query := fmt.Sprintf(
			"info_hash=%v"+
				"&peer_id=%v"+
				"&port=%v"+
				"&uploaded=%v"+
				"&downloaded=%v"+
				"&left=%v"+
				"&compact=%v"+
				"&no_peer_id=%v"+
				"&event=%v"+
				"numwant=%v"+
				"&ip=%v"+
				"&key=%v"+
				"&trackerid=%v",
			url.QueryEscape(string(req.Infohash[:])),
			url.QueryEscape(string(req.PeerID[:])),
			url.QueryEscape(strconv.Itoa(int(req.Port))),
			url.QueryEscape(strconv.Itoa(int(req.Uploaded))),
			url.QueryEscape(strconv.Itoa(int(req.Downloaded))),
			url.QueryEscape(strconv.Itoa(int(req.Left))),
			url.QueryEscape(strconv.Itoa(int(req.Compact))),
			url.QueryEscape(strconv.Itoa(int(req.NoPID))),
			url.QueryEscape(eventStr[req.Event]),
			url.QueryEscape(strconv.Itoa(int(req.Numwant))),
			url.QueryEscape(req.Ip.String()),
			url.QueryEscape(strconv.Itoa(int(req.Key))),
			url.QueryEscape(req.TrackerID),
		)
		trackerUrl.RawQuery = query
	} else {
		path := strings.Replace(trackerUrl.Path, "announce", "scrape", 1)
		if path == trackerUrl.Path {
			return Response{}, scrape_not_supported_err
		}
		trackerUrl.Path = path
		trackerUrl.RawQuery = fmt.Sprintf("info_hash=%v", url.QueryEscape(string(req.Infohash[:])))
	}
	httpResp, err := http.Get(trackerUrl.String())
	if err != nil {
		return Response{}, err
	}
	resp, err := m.parseHttp(httpResp, req)
	if err != nil {
		return Response{}, err
	}
	return resp, nil
}

func (m *manager) parseHttp(httpResp *http.Response, req Request) (Response, error) {
	content := []byte{}
	resp := Response{}

	content, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, err
	}

	decoded, err := bencode.Decode(string(content))
	if err != nil {
		return Response{}, err
	}
	root, ok := decoded.Dict()
	if !ok {
		return Response{}, invalid_tracker_resp_err
	}

	interval, _ := root.FindIntOrDef("interval", 1800)
	resp.Interval = uint32(interval)
	minInterval, _ := root.FindIntOrDef("min interval", 30)
	resp.MinInterval = uint32(minInterval)
	warning, warningOk := root.FindStr("warning reason")
	if warningOk {
		str := string(warning)
		resp.Warning = &str
	}
	failure, failureOk := root.FindStr("failure reason")
	if failureOk {
		str := string(failure)
		resp.Failure = &str
		return resp, nil
	}
	trackerId, trackeridOk := root.FindStr("tracker id")
	if trackeridOk {
		str := string(trackerId)
		resp.TrackerID = &str
		m.httpUrls[req.Url] = string(trackerId)
	}

	if req.Kind == Scrape {
		files, ok := root.FindDict("files")
		if !ok {
			return Response{}, invalid_tracker_resp_err
		}
		file, ok := files.FindDict(string(req.Infohash[:]))
		if !ok {
			return Response{}, invalid_tracker_resp_err
		}
		complete, _ := file.FindIntOrDef("complete", -1)
		incomplete, _ := file.FindIntOrDef("incomplete", -1)
		downloaded, _ := file.FindIntOrDef("downloaded", -1)
		resp.Complete = int64(complete)
		resp.Incomplete = int64(incomplete)
		resp.Downloaded = int64(downloaded)
		return resp, nil
	}
	complete, _ := root.FindIntOrDef("complete", -1)
	incomplete, _ := root.FindIntOrDef("incomplete", -1)
	downloaded, _ := root.FindIntOrDef("downloaded", -1)
	resp.Complete = int64(complete)
	resp.Incomplete = int64(incomplete)
	resp.Downloaded = int64(downloaded)

	peerList, ok := root.Find("peers")
	if !ok {
		return Response{}, invalid_tracker_resp_err
	}

	if peerList.Type() == bencode.List_t {
		peers, _ := peerList.List()
		for _, peerNode := range peers {
			peer, ok := peerNode.Dict()
			if !ok {
				return Response{}, invalid_tracker_resp_err
			}
			pid, _ := peer.FindStrOrDef("peer id", "")
			ip, _ := peer.FindStrOrDef("ip", "")
			port, _ := peer.FindIntOrDef("port", -1)

			if ip == "" || port == -1 {
				continue
			}
			parsedIp, err := netip.ParseAddr(string(ip))
			if err != nil {
				return Response{}, invalid_tracker_resp_err
			}

			peerVal := torrent.Peer{}
			copy(peerVal.PeerID[:], string(pid))
			peerVal.IpPort = netip.AddrPortFrom(parsedIp, uint16(port))
			resp.PeerList = append(resp.PeerList, peerVal)
		}
	} else if peerList.Type() == bencode.Str_t {
		peersStr, _ := peerList.Str()
		peers := []byte(peersStr)

		for len(peers) != 0 {
			ip := peers[0:4]
			port := peers[4:6]

			parsedIp, err := netip.ParseAddr(fmt.Sprintf("%v.%v.%v.%v", ip[3], ip[2], ip[1], ip[0]))
			if err != nil {
				return Response{}, invalid_tracker_resp_err
			}

			peerVal := torrent.Peer{}
			peerVal.IpPort = netip.AddrPortFrom(parsedIp, uint16(port[1])|uint16(port[0])<<8)
			resp.PeerList = append(resp.PeerList, peerVal)

			peers = peers[6:]
		}
	}

	return resp, nil
}
