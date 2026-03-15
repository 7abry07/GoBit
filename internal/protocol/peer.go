package protocol

import (
	"net/netip"
)

type Peer struct {
	Conn *PeerConnection

	FailureCnt      int
	TotalDownloaded int
	TotalUploaded   int

	Endpoint netip.AddrPort
	Banned   bool
}

func NewPeer(e netip.AddrPort) *Peer {
	peer := Peer{}

	peer.Endpoint = e
	peer.Conn = nil
	peer.FailureCnt = 0
	peer.TotalDownloaded = 0
	peer.TotalUploaded = 0

	return &peer
}
