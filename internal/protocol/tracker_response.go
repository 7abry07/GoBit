package protocol

import (
	"GoBit/internal/bencode"
	"fmt"
	"net/netip"
)

type TrackerResponse struct {
	Failure     *string
	Warning     *string
	TrackerID   *string
	MinInterval uint32
	Interval    uint32
	Complete    int64
	Incomplete  int64
	Downloaded  int64
	PeerList    []Peer
}

func ParseTrackerResponseHttp(httpResp []byte, req TrackerRequest) (TrackerResponse, error) {
	resp := TrackerResponse{}
	decoded, err := bencode.Decode(string(httpResp))
	if err != nil {
		return TrackerResponse{}, err
	}
	root, ok := decoded.Dict()
	if !ok {
		return TrackerResponse{}, Tracker_invalid_resp_err
	}

	interval, _ := root.FindIntOrDef("interval", 1800)
	minInterval, _ := root.FindIntOrDef("min interval", 30)
	resp.Interval = uint32(interval)
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
	}

	if req.Kind == TrackerScrape {
		files, ok := root.FindDict("files")
		if !ok {
			return TrackerResponse{}, Tracker_invalid_resp_err
		}
		file, ok := files.FindDict(string(req.Infohash[:]))
		if !ok {
			return TrackerResponse{}, Tracker_invalid_resp_err
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
		return TrackerResponse{}, Tracker_invalid_resp_err
	}

	if peerList.Type() == bencode.List_t {
		peers, _ := peerList.List()
		for _, peerNode := range peers {
			p, ok := peerNode.Dict()
			if !ok {
				return TrackerResponse{}, Tracker_invalid_resp_err
			}
			pid, _ := p.FindStrOrDef("peer id", "")
			ip, _ := p.FindStrOrDef("ip", "")
			port, _ := p.FindIntOrDef("port", -1)

			if ip == "" || port == -1 {
				continue
			}
			parsedIp, err := netip.ParseAddr(string(ip))
			if err != nil {
				return TrackerResponse{}, Tracker_invalid_resp_err
			}

			peerVal := Peer{}
			copy(peerVal.PeerID[:], string(pid))
			peerVal.IpPort = netip.AddrPortFrom(parsedIp, uint16(port))
			resp.PeerList = append(resp.PeerList, peerVal)
		}
	} else if peerList.Type() == bencode.Str_t {
		peersStr, _ := peerList.Str()
		peers := []byte(peersStr)

		lst, ok := parseV4CompactPeers(peers)
		if !ok {
			return TrackerResponse{}, Tracker_invalid_resp_err
		}
		resp.PeerList = append(resp.PeerList, lst...)
	}

	peer6List, ok := root.Find("peers6")
	if ok {
		if peer6List.Type() == bencode.Str_t {
			peersStr, _ := peer6List.Str()
			peers := []byte(peersStr)

			lst, ok := parseV6CompactPeers(peers)
			if ok {
				resp.PeerList = append(resp.PeerList, lst...)
			}
		}
	}

	return resp, nil
}

func parseV4CompactPeers(peers []byte) ([]Peer, bool) {
	peerList := []Peer{}

	for {
		if len(peers) == 0 {
			break
		}

		ip := peers[0:4]
		port := peers[4:6]

		parsedIp, err := netip.ParseAddr(fmt.Sprintf("%v.%v.%v.%v", ip[0], ip[1], ip[2], ip[3]))
		if err != nil {
			return []Peer{}, false
		}

		peerVal := Peer{}
		peerVal.IpPort = netip.AddrPortFrom(parsedIp, uint16(port[1])|uint16(port[0])<<8)
		peerList = append(peerList, peerVal)

		peers = peers[6:]
	}
	return peerList, true
}

func parseV6CompactPeers(peers []byte) ([]Peer, bool) {
	peerList := []Peer{}

	for {
		if len(peers) == 0 {
			break
		}

		ip := peers[0:16]
		port := peers[16:18]

		parsedIp, err := netip.ParseAddr(fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x",
			uint16(ip[1])|uint16(ip[0])<<8,
			uint16(ip[3])|uint16(ip[2])<<8,
			uint16(ip[5])|uint16(ip[4])<<8,
			uint16(ip[7])|uint16(ip[6])<<8,
			uint16(ip[9])|uint16(ip[8])<<8,
			uint16(ip[11])|uint16(ip[10])<<8,
			uint16(ip[13])|uint16(ip[12])<<8,
			uint16(ip[15])|uint16(ip[14])<<8))

		if err != nil {
			return []Peer{}, false
		}

		peerVal := Peer{}
		peerVal.IpPort = netip.AddrPortFrom(parsedIp, uint16(port[1])|uint16(port[0])<<8)
		peerList = append(peerList, peerVal)

		peers = peers[18:]
	}
	return peerList, true
}
