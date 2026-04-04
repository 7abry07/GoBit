package main

import (
	"GoBit/internal/protocol"
	"time"
)

func main() {
	name := "fedora"
	file := protocol.TorrentFile{}

	file, err := protocol.ParseTorrentFile("internal/tests/protocol/test_files/" + name + ".torrent")
	if err != nil {
		panic(err)
	}

	ses := protocol.NewSession()
	torr := protocol.NewTorrent(file, ses)

	ses.AddTorrent(torr)
	ses.Start()

	// time.Sleep(90 * time.Second)
	// ses.StopTorrent(torr)

	time.Sleep(time.Hour * 10)
}
