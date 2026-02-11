package torrent

import "net/netip"

type Peer struct {
	PeerID [20]byte
	IpPort netip.AddrPort
}
