package protocol

import (
	"context"
	"net/netip"
)

type Peer struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	Conn   *PeerConnection

	Endpoint netip.AddrPort

	FailureCnt      int
	TotalDownloaded int
	TotalUploaded   int
}

func NewPeer(t *Torrent, e netip.AddrPort) *Peer {
	peer := Peer{}

	peer.ctx, peer.cancel = context.WithCancelCause(t.ctx)
	peer.Conn = nil

	peer.Endpoint = e

	peer.FailureCnt = 0
	peer.TotalDownloaded = 0
	peer.TotalUploaded = 0

	return &peer
}
