package protocol

import "net/netip"

type Peer struct {
	Pid    PeerID
	IpPort netip.AddrPort
}
