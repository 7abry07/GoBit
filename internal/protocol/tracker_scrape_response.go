package protocol

import (
	"GoBit/internal/bencode"
	"encoding/binary"
	"fmt"
)

type TorrentStats struct {
	Incomplete uint32
	Complete   uint32
	Downloaded uint32
}

type TrackerScrapeResponse struct {
	Stats map[[20]byte]TorrentStats
}

func (res *TrackerScrapeResponse) DeserializeUdp(t *UdpTracker, req TrackerScrapeRequest, udpResp []byte) error {
	res.Stats = make(map[[20]byte]TorrentStats)
	action := binary.BigEndian.Uint32(udpResp[0:4])

	switch byte(action) {
	case byte(SCRAPE):
		{
			if (len(udpResp)-8)/12 != len(req.infohashes) {
				return Tracker_invalid_resp_err
			}

			for _, infohash := range req.infohashes {
				stats := TorrentStats{}
				seeders := binary.BigEndian.Uint32(udpResp[8:12])
				completed := binary.BigEndian.Uint32(udpResp[12:16])
				leechers := binary.BigEndian.Uint32(udpResp[16:20])

				stats.Incomplete = leechers
				stats.Complete = seeders
				stats.Downloaded = completed

				res.Stats[infohash] = stats
				udpResp = udpResp[20:]
			}
		}
	case byte(ERROR):
		{
			return fmt.Errorf("error in tracker response: %v", string(udpResp[8:]))
		}
	default:
		return Tracker_invalid_resp_err
	}

	return nil
}

func (res *TrackerScrapeResponse) DeserializeHttp(t *HttpTracker, httpResp []byte, req TrackerScrapeRequest) error {
	decoded, err := bencode.Decode(string(httpResp))
	if err != nil {
		return Tracker_invalid_resp_err
	}
	root, ok := decoded.Dict()
	if !ok {
		return Tracker_invalid_resp_err
	}

	files, ok := root.FindDict("files")
	if !ok {
		return Tracker_invalid_resp_err
	}

	for _, infohash := range req.infohashes {
		decodedStats, ok := files.FindDict(string(infohash[:]))
		if !ok {
			return Tracker_invalid_resp_err
		}

		stats := TorrentStats{}
		complete, _ := decodedStats.FindIntOrDef("complete", -1)
		incomplete, _ := decodedStats.FindIntOrDef("incomplete", -1)
		downloaded, _ := decodedStats.FindIntOrDef("downloaded", -1)

		if complete == -1 || incomplete == -1 || downloaded == -1 {
			return Tracker_invalid_resp_err
		}
		stats.Complete = uint32(complete)
		stats.Incomplete = uint32(incomplete)
		stats.Incomplete = uint32(downloaded)
		res.Stats[infohash] = stats
	}

	return nil
}
