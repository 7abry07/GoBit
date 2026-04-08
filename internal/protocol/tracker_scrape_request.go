package protocol

import (
	"encoding/binary"
	"net/url"
	"strings"
)

type TrackerScrapeRequest struct {
	infohashes [][20]byte
}

func (req TrackerScrapeRequest) SerializeHttp(t *HttpTracker) url.URL {
	fullUrl := t.scrape
	query := strings.Builder{}

	if len(req.infohashes) > 74 {
		panic("too many torrents in a single scrape")
	}

	for _, infohash := range req.infohashes {
		query.WriteString("info_hash=" + url.QueryEscape(string(infohash[:])))
	}
	fullUrl.RawQuery = query.String()
	return fullUrl
}

func (req TrackerScrapeRequest) SerializeUdp(t *UdpTracker, transactionId uint32) []byte {
	buf := []byte{}
	buf = binary.BigEndian.AppendUint64(buf, t.connectionId)
	buf = binary.BigEndian.AppendUint32(buf, uint32(SCRAPE))
	buf = binary.BigEndian.AppendUint32(buf, transactionId)

	if len(req.infohashes) > 74 {
		panic("too many torrents in a single scrape")
	}

	for _, infohash := range req.infohashes {
		buf = append(buf, infohash[:]...)
	}

	return buf
}
