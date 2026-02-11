package tracker

import "GoBit/internal/torrent"

type Response struct {
	Failure     *string
	Warning     *string
	TrackerID   *string
	MinInterval uint32
	Interval    uint32
	Complete    int64
	Incomplete  int64
	Downloaded  int64
	PeerList    []torrent.Peer
}
