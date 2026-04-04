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

	PendingRequests []BlockRequest

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

	p.Conn.SendRequest(req)
	p.State.PendingRequests = append(p.State.PendingRequests, req)
	go req.StartTimeout()

	//
	// fmt.Printf("REQUESTED (%v:%v:%v) FROM %v\n", idx, begin, p.Conn.torrent.Picker.GetBlockSize(idx, begin), p.Conn.Pid)
	//

	p.Conn.torrent.Picker.setBlockState(idx, begin, BLOCK_REQUESTED)
	p.Conn.torrent.Picker.setPieceState(idx, PIECE_DOWNLOADING)
	p.Conn.torrent.Picker.blocksInFlight++
}

func (p *ActivePeer) Piece(idx, begin uint32, data []byte) {
	p.Conn.SendBlock(idx, begin, data)
}

func (p *ActivePeer) Cancel(idx, begin, length uint32) {
	timer := time.NewTimer(0)
	<-timer.C
	cancelReq := NewBlockRequest(nil, p.Conn.Pid, idx, begin, length)

	for i, req := range p.State.PendingRequests {
		if CompareRequests(req, cancelReq) {
			p.State.PendingRequests[i] = p.State.PendingRequests[len(p.State.PendingRequests)-1]
			p.State.PendingRequests = p.State.PendingRequests[:len(p.State.PendingRequests)-1]
			p.Conn.SendCancel(cancelReq)
			return
		}
	}
}

func (p *ActivePeer) HasPiece(idx uint32) bool {
	return p.State.Pieces.Test(uint(idx))
}
func (p *ActivePeer) FillOutstandingRequest() {
	for len(p.State.PendingRequests) < 10 {
		newIdx, begin, ok := p.Conn.torrent.Picker.Pick(*p.State)
		if !ok {
			//
			//
			// fmt.Println("NOTHING TO REQUEST")
			// for i := range p.State.Pieces.EachSet() {
			// 	if !p.Conn.torrent.bitfield.Test(i) {
			// 		fmt.Printf("piece %v is %v [",
			// 			i,
			// 			p.Conn.torrent.Picker.pieces[i].state)
			// 		for _, block := range p.Conn.torrent.Picker.pieces[i].blocks {
			// 			fmt.Printf("%v", block.state)
			// 		}
			// 		fmt.Println("]")
			// 	}
			// }
			// panic("")
			//
			//
			return
		}

		p.Request(newIdx, begin,
			p.Conn.torrent.Picker.GetBlockSize(newIdx, begin))
	}
}

func (p *ActivePeer) ClearOutstandingRequests() {
	for _, req := range p.State.PendingRequests {
		p.Conn.torrent.SignalEvent(RescheduleBlock{p.Conn.Pid, req.Idx, req.Begin, req.Length})
		// p.Conn.torrent.RescheduleBlock(req, p.Conn.Pid)
		req.Received()
	}
	p.State.PendingRequests = nil
}
