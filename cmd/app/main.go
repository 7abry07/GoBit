package main

import (
	"GoBit/internal/protocol"
	"fmt"
	"time"
)

func main() {
	file, err := protocol.ParseTorrentFile("internal/tests/protocol/test_files/naruto.torrent")
	if err != nil {
		panic(err)
	}

	ses := protocol.NewSession()
	torr := protocol.NewTorrent(file, ses)

	ses.Start()
	ses.AddTorrent(torr)

	time.Sleep(7 * time.Second)
	fmt.Println("EXPIRED")
	ses.Stop()
	for {
	}
}
