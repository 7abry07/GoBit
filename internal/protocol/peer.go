package protocol

import (
	"net/netip"
)

type Peer struct {
	Endpoint netip.AddrPort

	Conn *PeerConnection

	FailureCnt          int
	PrevTotalDownloaded int
	PrevTotalUploaded   int
}

func NewPeer(e netip.AddrPort) *Peer {
	peer := Peer{}

	peer.Conn = nil
	peer.Endpoint = e
	peer.FailureCnt = 0
	peer.PrevTotalDownloaded = 0
	peer.PrevTotalUploaded = 0

	return &peer
}
