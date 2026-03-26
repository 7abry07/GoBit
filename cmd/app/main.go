package main

import (
	"GoBit/internal/protocol"
	// "time"
	"net/http"
	_ "net/http/pprof"
)

func main() {
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	name := "one_piece"
	file := protocol.TorrentFile{}

	file, err := protocol.ParseTorrentFile("internal/tests/protocol/test_files/" + name + ".torrent")
	if err != nil {
		panic(err)
	}

	ses := protocol.NewSession()
	torr := protocol.NewTorrent(file, ses)

	ses.Start()
	ses.AddTorrent(torr)

	// time.Sleep(30 * time.Second)
	// ses.StopTorrent(torr)

	for {
	}

}
