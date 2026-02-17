package main

import (
// "GoBit/internal/protocol"
// "GoBit/internal/tracker"
// "fmt"
)

func main() {
	// file, err := protocol.ParseTorrentFile("internal/tests/protocol/test_files/naruto.torrent")
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
	// peerID := []byte("-GB0001-123456789012")
	//
	// go man.Send(req)
	//
	// resp := <-ch
	// if resp.Err != nil {
	// 	panic(resp.Err)
	// }
	//
	// conns := []protocol.PeerConnection{}
	// for _, peer := range resp.Val.PeerList {
	// 	fmt.Printf("<%v> : [%v]\n", peer.IpPort.Addr(), peer.IpPort.Port())
	// 	conn, err := protocol.NewPeerConn(peer, file.InfoHash, [20]byte(peerID))
	// 	if err == nil {
	// 		conns = append(conns, conn)
	// 	}
	// }
	//
	// for _, conn := range conns {
	// 	fmt.Printf("connected -> %v\n", conn.PeerInfo.IpPort.String())
	// }
	//
	// for _, peer := range conns {
	// 	msg := <-peer.In
	// 	fmt.Printf("message type -> %v\nmessage length-> %v\n\n", msg.Kind, len(msg.Payload)*8)
	// }
}
