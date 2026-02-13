package main

import (
	// "GoBit/internal/torrent"
	"GoBit/internal/tracker"
	// "fmt"
)

func main() {
	// file, err := torrent.ParseFile("internal/tests/torrent/test_files/naruto.torrent")
	// man, ch := tracker.NewManager()
	// if err != nil {
	// 	panic(err)
	// }
	//
	// req := tracker.Request{}
	//
	// req.PeerID = [20]byte{}
	// req.Infohash = file.InfoHash
	// req.Event = tracker.Started
	// req.Url = file.Announce
	//
	// go man.Send(req)
	//
	// resp := <-ch
	// if resp.Err != nil {
	// 	panic(resp.Err)
	// }
	//
	// for _, peer := range resp.Val.PeerList {
	// 	fmt.Printf("<%v> : [%v]\n", peer.IpPort.Addr(), peer.IpPort.Port())
	// }

	req := tracker.Request{}
	resp := "d5:filesd20:00000000000000000000d8:completei0e10:downloadedi0e10:incompletei0eeee"

	req.Kind = tracker.Scrape
	req.Infohash = [20]byte{0}

	tracker.ParseHttp([]byte(resp), req)
}
