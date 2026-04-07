package protocol

import (
	// "fmt"
	"github.com/bits-and-blooms/bitset"
	"time"
)

type ActivePeer struct {
	Conn  *PeerConnection
	State *ActivePeerState
}

type ActivePeerState struct {
	LastTickTime     time.Time
	TotalDownloaded  int
	TotalUploaded    int
	LastTickDownload int
	LastTickUpload   int
	DownloadRate     float64
	UploadRate       float64

	Pieces *bitset.BitSet

	PendingRequests  []BlockRequest
	IncomingRequests []PeerRequest

	IsChoked      bool
	IsInteresting bool
	AmChoked      bool
	AmInteresting bool
	IsOptimistic  bool
	IsSeed        bool
}

func (p *ActivePeer) KeepAlive() {
	p.Conn.SendKeepAlive()
}

func (p *ActivePeer) Choke() {
	if p.State.IsChoked {
		return
	}
	p.Conn.SendChoked(true)
	p.State.IsChoked = true
}

func (p *ActivePeer) Unchoke() {
	if !p.State.IsChoked {
		return
	}
	p.Conn.SendChoked(false)
	p.State.IsChoked = false
}

func (p *ActivePeer) SetInteresting() {
	if p.State.IsInteresting {
		return
	}
	p.Conn.SendInterested(true)
	p.State.IsInteresting = true

	if !p.State.IsChoked {
		p.FillOutstandingRequest()
	}
}

func (p *ActivePeer) SetUninteresting() {
	if !p.State.IsInteresting {
		return
	}
	p.Conn.SendInterested(false)
	p.State.IsInteresting = false
}

func (p *ActivePeer) Have(idx uint32) {
	p.Conn.SendHave(idx)
}

func (p *ActivePeer) Bitfield(bf bitset.BitSet) {
	p.Conn.SendBitfield(bf)
}

func (p *ActivePeer) Request(idx, begin, length uint32) {
	req := NewBlockRequest(p.Conn.torrent, p.Conn.Pid, idx, begin, length)

	p.Conn.SendRequest(idx, begin, length)
	p.State.PendingRequests = append(p.State.PendingRequests, req)
	go req.StartTimeout()

	//
	// fmt.Printf("REQUESTED (%v:%v:%v) FROM %v\n", idx, begin, p.Conn.torrent.Picker.GetBlockSize(idx, begin), p.Conn.Pid)
	//

	p.Conn.torrent.Picker.setBlockState(idx, begin, BLOCK_REQUESTED)
	p.Conn.torrent.Picker.setPieceState(idx, PIECE_DOWNLOADING)

	//
	p.Conn.torrent.Picker.BlocksToRequestDec()
	//
}

func (p *ActivePeer) Piece(idx, begin uint32, data []byte) {
	p.Conn.SendBlock(idx, begin, data)
}

func (p *ActivePeer) Cancel(idx, begin, length uint32) {
	ok := p.RemoveRequest(idx, begin, length)
	if ok {
		p.Conn.SendCancel(idx, begin, length)
	}
}

func (p *ActivePeer) RemoveRequest(idx, begin, length uint32) bool {
	for i, r := range p.State.PendingRequests {
		if idx == r.Idx && begin == r.Begin && length == r.Length {
			p.State.PendingRequests[i] = p.State.PendingRequests[len(p.State.PendingRequests)-1]
			p.State.PendingRequests = p.State.PendingRequests[:len(p.State.PendingRequests)-1]
			r.Received()
			return true
		}
	}
	return false
}

func (p *ActivePeer) RemoveIncomingRequest(idx, begin, length uint32) bool {
	for i, r := range p.State.IncomingRequests {
		if idx == r.Idx && begin == r.Begin && length == r.Length {
			p.State.IncomingRequests[i] = p.State.IncomingRequests[len(p.State.IncomingRequests)-1]
			p.State.IncomingRequests = p.State.IncomingRequests[:len(p.State.IncomingRequests)-1]
			return true
		}
	}
	return false
}

func (p *ActivePeer) RequestCanceled(idx, begin, length uint32) bool {
	for _, r := range p.State.IncomingRequests {
		if idx == r.Idx && begin == r.Begin && length == r.Length {
			return false
		}
	}
	return true
}

func (p *ActivePeer) FillOutstandingRequest() {
	for len(p.State.PendingRequests) < 10 {
		idx, begin, length, ok := p.Conn.torrent.Picker.Pick(*p.State)
		if !ok {
			return
		}

		for _, req := range p.State.PendingRequests {
			if req.Idx == idx && req.Begin == begin && req.Length == length {
				return
			}
		}

		p.Request(idx, begin, length)
	}
}

func (p *ActivePeer) ClearOutstandingRequests() {
	for _, req := range p.State.PendingRequests {
		p.RemoveRequest(req.Idx, req.Begin, req.Length)
		p.Conn.torrent.SignalEvent(RescheduleBlock{p.Conn.Pid, req.Idx, req.Begin, req.Length})
	}
	p.State.PendingRequests = nil
}

func (p *ActivePeer) HasPiece(idx uint32) bool {
	return p.State.Pieces.Test(uint(idx))
}
