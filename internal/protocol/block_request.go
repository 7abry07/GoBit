package protocol

import (
	// "fmt"
	"time"
)

type BlockRequest struct {
	To      PeerID
	Idx     uint32
	Begin   uint32
	Length  uint32
	timeout *time.Timer
	done    chan struct{}

	torrent *Torrent
}

func NewBlockRequest(torrent *Torrent, receiver PeerID, idx, begin, length uint32) BlockRequest {
	return BlockRequest{
		receiver, idx, begin, length, nil, make(chan struct{}), torrent,
	}
}

func CompareRequests(req1, req2 BlockRequest) bool {
	return req1.Idx == req2.Idx &&
		req1.Begin == req2.Begin &&
		req1.Length == req2.Length
}

func (r *BlockRequest) StartTimeout() {
	r.timeout = time.NewTimer(time.Second * 10)
	select {
	case <-r.timeout.C:
		// fmt.Println("FIRED")
		r.torrent.SignalEvent(RequestTimeout{r.To, r.Idx, r.Begin, r.Length})
	case <-r.done:
		if !r.timeout.Stop() {
			<-r.timeout.C
		}
		return
	}
}

func (r *BlockRequest) Received() {
	// r.done <- struct{}{}
	close(r.done)
}
