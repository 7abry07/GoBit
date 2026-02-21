package protocol

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

type PeerManager struct {
	PeerId         [20]byte
	Port           uint16
	listener       net.Listener
	peerList       []PeerConnection
	activeTorrents map[[20]byte]*Torrent
}

func NewPeerManager() *PeerManager {
	man := PeerManager{}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	portStart := strings.LastIndex(listener.Addr().String(), ":")
	if portStart == -1 {
		panic(err)
	}

	port, err := strconv.Atoi(listener.Addr().String()[portStart+1:])
	if err != nil {
		panic(err)
	}
	man.Port = uint16(port)
	man.listener = listener
	man.PeerId = generateRandomPeerId()
	man.activeTorrents = make(map[[20]byte]*Torrent)

	fmt.Printf("PORT -> %v\n", man.Port)

	go man.listen()
	return &man
}

func (m *PeerManager) DialPeers(torrent *Torrent, peers []Peer) {
	for _, p := range peers {
		go m.DialPeer(torrent, p)
	}
}

func (m *PeerManager) listen() {
	for {
		conn, err := m.listener.Accept()
		if err != nil {
			conn.Close()
			continue
		}
		go m.handshakePeer(conn)
	}
}

func (m *PeerManager) AddTorrent(t *Torrent) {
	m.activeTorrents[t.Info.InfoHash] = t
}

func (m *PeerManager) DialPeer(t *Torrent, p Peer) {
	conn, err := NewPeerConn(p, t.Info.InfoHash, m.PeerId, t)
	if err != nil {
		fmt.Printf("CONNECTION FAILED -> %v\n", err.Error())
		return
	}
	fmt.Printf("CONNECTION SUCCESS -> %v\n", conn.PeerInfo.IpPort.String())
	t.NewPeer <- conn
}

func (m *PeerManager) handshakePeer(conn net.Conn) {
	buf := make([]byte, 68)
	bytesRead, err := io.ReadFull(conn, buf)
	if err != nil || bytesRead != 68 {
		conn.Close()
		return
	}
	if string(buf[0:20]) != "\x13BitTorrent protocol" {
		conn.Close()
		return
	}

	infohash := [20]byte(buf[29:49])
	pid := [20]byte(buf[49:])

	torrent, found := m.activeTorrents[infohash]
	if !found {
		conn.Close()
		return
	}

	sendHandshake(conn, infohash, m.PeerId)

	endpoint, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		panic(err)
	}
	peerInfo := Peer{PeerID: pid, IpPort: endpoint}

	peerConn := PeerConnection{}
	peerConn.PeerInfo = peerInfo
	peerConn.sock = conn
	peerConn.bitfieldSent = false
	peerConn.pieces = nil
	peerConn.torr = torrent
	peerConn.AmChoked = true
	peerConn.IsChoked = true
	peerConn.AmInteresting = false
	peerConn.IsInterested = false
	peerConn.Out = make(chan PeerMessage)

	go peerConn.readLoop()
	go peerConn.writeLoop()

	torrent.NewPeer <- &peerConn
	m.peerList = append(m.peerList, peerConn)
}

func generateRandomPeerId() [20]byte {
	pid := []byte{}
	pid = append(pid, []byte("-GB0001-")...)

	randomNumbers := make([]byte, 12)
	for i := range randomNumbers {
		randomNumbers[i] = byte(rand.Intn(10) + '0')
	}

	pid = append(pid, randomNumbers...)

	return [20]byte(pid)
}
