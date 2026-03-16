package main

import (
	"GoBit/internal/protocol"
	"bufio"
	"fmt"
	"os"
	// "time"
)

func main() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("1.Naruto\n2.One Piece\n3.Crime101\n\n-> ")
	opt, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	file := protocol.TorrentFile{}

	if opt == "1\n" {
		file, err = protocol.ParseTorrentFile("internal/tests/protocol/test_files/naruto.torrent")
	} else if opt == "2\n" {
		file, err = protocol.ParseTorrentFile("internal/tests/protocol/test_files/one_piece.torrent")
	} else if opt == "3\n" {
		file, err = protocol.ParseTorrentFile("internal/tests/protocol/test_files/crime101.torrent")
	} else {
		panic("invalid option")
	}

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
