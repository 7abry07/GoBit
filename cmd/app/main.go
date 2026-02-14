package main

import (
	"GoBit/internal/torrent"
	"GoBit/internal/tracker"
	"fmt"
)

func main() {
	file, err := torrent.ParseFile("internal/tests/torrent/test_files/naruto.torrent")
	man, ch := tracker.NewManager()
	if err != nil {
		panic(err)
	}

	req := tracker.Request{}

	req.PeerID = [20]byte{}
	req.Infohash = file.InfoHash
	req.Event = tracker.Started
	req.Url = file.Announce

	peerID := []byte("-GT0001-123456789012")

	go man.Send(req)

	resp := <-ch
	if resp.Err != nil {
		panic(resp.Err)
	}

	conns := []torrent.PeerConnection{}
	for _, peer := range resp.Val.PeerList {
		fmt.Printf("<%v> : [%v]\n", peer.IpPort.Addr(), peer.IpPort.Port())
		conn, err := torrent.NewPeerConn(peer, file.InfoHash, [20]byte(peerID))
		if err == nil {
			conns = append(conns, conn)
		}
	}

	for _, conn := range conns {
		fmt.Println(conn.PeerInfo.IpPort.String())
	}
}
