package main

import (
	"GoBit/internal/protocol"
	"time"
)

func main() {
	name := "debian"
	file := protocol.TorrentFile{}

	file, err := protocol.ParseTorrentFile("internal/tests/protocol/test_files/" + name + ".torrent")
	if err != nil {
		panic(err)
	}

	ses := protocol.NewSession()
	torr := protocol.NewTorrent(file, ses)

	ses.AddTorrent(torr)
	ses.Start()

	// time.Sleep(30 * time.Second)
	// ses.StopTorrent(torr)

	time.Sleep(time.Hour * 10)
}
