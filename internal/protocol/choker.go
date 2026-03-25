package protocol

import "slices"

func comparePeersUpload(p1, p2 ActivePeerState) int {
	if p1.UploadRate > p2.UploadRate {
		return 1
	} else if p1.UploadRate > p2.UploadRate {
		return -1
	}
	return 0
}

func UnchokeSort(peers []ActivePeer) int {
	peersToUnchoke := 4
	slices.SortFunc(peers, func(a, b ActivePeer) int {
		if a.State.AmInteresting && !b.State.AmInteresting {
			return 1
		} else if a.State.AmInteresting && !b.State.AmInteresting {
			return -1
		}
		return comparePeersUpload(*a.State, *b.State)
	})
	return min(peersToUnchoke, len(peers))
}
