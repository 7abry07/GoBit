package main

import (
	"GoBit/internal/protocol"
)

func main() {
	file, err := protocol.ParseTorrentFile("internal/tests/protocol/test_files/naruto.torrent")
	if err != nil {
		panic(err)
	}
	peerMan := protocol.NewPeerManager()
	torr := protocol.NewTorrent(file, peerMan)
	peerMan.AddTorrent(torr)

	torr.Start()

	for {
	}
}
