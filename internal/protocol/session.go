package protocol

import (
	"GoBit/internal/utils"
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

type Session struct {
	ctx    context.Context
	cancel context.CancelFunc

	incomingPeer chan net.Conn

	listener net.Listener
	PeerID   [20]byte
	Port     uint16
	Torrents map[[20]byte]*Torrent
}

func NewSession() *Session {
	s := Session{}

	ctx := context.Background()
	s.ctx, s.cancel = context.WithCancel(ctx)

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
	s.listener = listener
	s.Port = uint16(port)
	s.Torrents = make(map[[20]byte]*Torrent)
	s.PeerID = utils.GenerateRandomPeerId()

	fmt.Printf("PORT -> %v\n", s.Port)

	return &s
}

func (s *Session) Start() {
	go s.loop()
}

func (s *Session) Stop() {
	s.cancel()
}

func (s *Session) loop() {
	go s.listenForPeers()
	for {
		select {
		case <-s.ctx.Done():
			{
				fmt.Println("SESSION STOPPED")
				s.listener.Close()
				return
			}
		case conn := <-s.incomingPeer:
			{
				go s.handshakePeer(conn)
			}
		}
	}
}

func (s *Session) listenForPeers() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			continue
		}
		s.incomingPeer <- conn
	}
}

func (s *Session) AddTorrent(t *Torrent) {
	s.Torrents[t.Info.InfoHash] = t
	go t.Start()
}

func (s *Session) RemoveTorrent(t *Torrent) {
	delete(s.Torrents, t.Info.InfoHash)
}

func (s *Session) handshakePeer(conn net.Conn) {
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

	torrent, found := s.Torrents[infohash]
	if !found {
		conn.Close()
		return
	}

	_, err = utils.SendHandshake(conn, infohash, s.PeerID)
	if err != nil {
		conn.Close()
		return
	}

	endpoint, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		panic(err)
	}
	peerInfo := Peer{Pid: pid, IpPort: endpoint}

	peerConn := peerConnection{}
	peerConn.ctx, peerConn.cancel = context.WithCancel(torrent.ctx)
	peerConn.Info = peerInfo
	peerConn.sock = conn
	peerConn.bitfieldSent = false
	peerConn.pieces = make([]uint8, len(torrent.Info.Pieces)/20)
	peerConn.AmChoked = true
	peerConn.IsChoked = true
	peerConn.AmInteresting = false
	peerConn.IsInteresting = false
	peerConn.Out = make(chan PeerMessage)
	peerConn.in = make(chan PeerMessage)
	peerConn.torr = torrent

	go peerConn.loop()

	torrent.NewPeer <- &peerConn
}
