package main

import (
	"GoBit/internal/protocol"
	// "time"
)

func main() {
	name := "naruto"
	file := protocol.TorrentFile{}

	file, err := protocol.ParseTorrentFile("internal/tests/protocol/test_files/" + name + ".torrent")
	if err != nil {
		panic(err)
	}

	ses := protocol.NewSession()
	torr := protocol.NewTorrent(file, ses)

	ses.Start()
	ses.AddTorrent(torr)

	// time.Sleep(15 * time.Second)
	// fmt.Println("EXPIRED")
	// ses.StopTorrent(torr)

	for {
	}

}
