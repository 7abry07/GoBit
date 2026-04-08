package protocol

import (
	"GoBit/internal/bencode"
	"encoding/binary"
	"fmt"
	"net/netip"
)

type TrackerAnnounceResponse struct {
	Failure     *string
	Warning     *string
	trackerID   *string
	MinInterval uint32
	Interval    uint32
	Complete    int64
	Incomplete  int64
	Downloaded  int64
	PeerList    []PeerEntry
}

func (res *TrackerAnnounceResponse) DeserializeUdp(t *UdpTracker, udpResp []byte) error {
	action := binary.BigEndian.Uint32(udpResp[0:4])
	switch byte(action) {
	case byte(ANNOUNCE):
		{
			interval := binary.BigEndian.Uint32(udpResp[8:12])
			leechers := binary.BigEndian.Uint32(udpResp[12:16])
			seeders := binary.BigEndian.Uint32(udpResp[16:20])

			res.Interval = interval
			res.Incomplete = int64(leechers)
			res.Complete = int64(seeders)

			peerList := []PeerEntry{}
			peerCnt := (len(udpResp) - 20) / 6
			udpResp = udpResp[20:]
			for range peerCnt {
				ip := udpResp[0:4]
				port := udpResp[4:6]

				parsedIp, err := netip.ParseAddr(fmt.Sprintf("%v.%v.%v.%v", ip[0], ip[1], ip[2], ip[3]))
				if err != nil {
					return Tracker_invalid_resp_err
				}

				peerVal := PeerEntry{}
				peerVal.IpPort = netip.AddrPortFrom(parsedIp, uint16(port[1])|uint16(port[0])<<8)
				peerList = append(peerList, peerVal)

				udpResp = udpResp[6:]
			}
		}
	case byte(ERROR):
		{
			return fmt.Errorf("error in tracker response: %v", udpResp[8:])
		}
	default:
		return Tracker_invalid_resp_err
	}

	return nil
}

func (res *TrackerAnnounceResponse) DeserializeHttp(httpResp []byte) error {
	decoded, err := bencode.Decode(string(httpResp))
	if err != nil {
		return err
	}
	root, ok := decoded.Dict()
	if !ok {
		return Tracker_invalid_resp_err
	}

	interval, _ := root.FindIntOrDef("interval", 1800)
	minInterval, _ := root.FindIntOrDef("min interval", 30)
	res.Interval = uint32(interval)
	res.MinInterval = uint32(minInterval)

	warning, warningOk := root.FindStr("warning reason")
	if warningOk {
		str := string(warning)
		res.Warning = &str
	}
	failure, failureOk := root.FindStr("failure reason")
	if failureOk {
		str := string(failure)
		res.Failure = &str
		return nil
	}
	trackerId, trackeridOk := root.FindStr("tracker id")
	if trackeridOk {
		str := string(trackerId)
		res.trackerID = &str
	}

	// if req.Kind == TrackerScrape {
	// 	files, ok := root.FindDict("files")
	// 	if !ok {
	// 		return TrackerAnnounceResponse{}, Tracker_invalid_resp_err
	// 	}
	// 	file, ok := files.FindDict(string(req.Infohash[:]))
	// 	if !ok {
	// 		return TrackerAnnounceResponse{}, Tracker_invalid_resp_err
	// 	}
	// 	complete, _ := file.FindIntOrDef("complete", -1)
	// 	incomplete, _ := file.FindIntOrDef("incomplete", -1)
	// 	downloaded, _ := file.FindIntOrDef("downloaded", -1)
	// 	resp.Complete = int64(complete)
	// 	resp.Incomplete = int64(incomplete)
	// 	resp.Downloaded = int64(downloaded)
	// 	return resp, nil
	// }
	complete, _ := root.FindIntOrDef("complete", -1)
	incomplete, _ := root.FindIntOrDef("incomplete", -1)
	downloaded, _ := root.FindIntOrDef("downloaded", -1)
	res.Complete = int64(complete)
	res.Incomplete = int64(incomplete)
	res.Downloaded = int64(downloaded)

	peerList, ok := root.Find("peers")
	if !ok {
		return Tracker_invalid_resp_err
	}

	if peerList.Type() == bencode.List_t {
		peers, _ := peerList.List()
		for _, peerNode := range peers {
			p, ok := peerNode.Dict()
			if !ok {
				return Tracker_invalid_resp_err
			}
			pid, _ := p.FindStrOrDef("peer id", "")
			ip, _ := p.FindStrOrDef("ip", "")
			port, _ := p.FindIntOrDef("port", -1)

			if ip == "" || port == -1 {
				continue
			}
			parsedIp, err := netip.ParseAddr(string(ip))
			if err != nil {
				return Tracker_invalid_resp_err
			}

			peerVal := PeerEntry{}
			peerVal.Pid, err = NewPeerID(([]byte)(pid))
			if err != nil {
				panic(err)
			}
			peerVal.IpPort = netip.AddrPortFrom(parsedIp, uint16(port))
			res.PeerList = append(res.PeerList, peerVal)
		}
	} else if peerList.Type() == bencode.Str_t {
		peersStr, _ := peerList.Str()
		peers := []byte(peersStr)

		lst, ok := parseV4CompactPeers(peers)
		if !ok {
			return Tracker_invalid_resp_err
		}
		res.PeerList = append(res.PeerList, lst...)
	}

	peer6List, ok := root.Find("peers6")
	if ok {
		if peer6List.Type() == bencode.Str_t {
			peersStr, _ := peer6List.Str()
			peers := []byte(peersStr)

			lst, ok := parseV6CompactPeers(peers)
			if ok {
				res.PeerList = append(res.PeerList, lst...)
			}
		}
	}

	return nil
}

func parseV4CompactPeers(peers []byte) ([]PeerEntry, bool) {
	peerList := []PeerEntry{}

	for {
		if len(peers) == 0 {
			break
		}

		ip := peers[0:4]
		port := peers[4:6]

		parsedIp, err := netip.ParseAddr(fmt.Sprintf("%v.%v.%v.%v", ip[0], ip[1], ip[2], ip[3]))
		if err != nil {
			return []PeerEntry{}, false
		}

		peerVal := PeerEntry{}
		peerVal.IpPort = netip.AddrPortFrom(parsedIp, uint16(port[1])|uint16(port[0])<<8)
		peerList = append(peerList, peerVal)

		peers = peers[6:]
	}
	return peerList, true
}

func parseV6CompactPeers(peers []byte) ([]PeerEntry, bool) {
	peerList := []PeerEntry{}

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
			uint16(ip[9])|uint16(ip[8])<<8, uint16(ip[11])|uint16(ip[10])<<8,
			uint16(ip[13])|uint16(ip[12])<<8,
			uint16(ip[15])|uint16(ip[14])<<8))

		if err != nil {
			return []PeerEntry{}, false
		}

		peerVal := PeerEntry{}
		peerVal.IpPort = netip.AddrPortFrom(parsedIp, uint16(port[1])|uint16(port[0])<<8)
		peerList = append(peerList, peerVal)

		peers = peers[18:]
	}
	return peerList, true
}
