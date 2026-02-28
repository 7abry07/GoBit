package protocol

import (
	"GoBit/internal/utils"
	"context"
	"fmt"
	"net"
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

func (s *Session) Start() {
	go s.loop()
}

func (s *Session) Stop() {
	s.cancel()
}

func (s *Session) AddTorrent(t *Torrent) {
	s.Torrents[t.Info.InfoHash] = t
	go t.Start()
}

func (s *Session) StopTorrent(t *Torrent) {
	t.cancel()
	delete(s.Torrents, t.Info.InfoHash)
}

func (s *Session) handshakePeer(conn net.Conn) {
	peerConn, err := newIncomingPeerConnection(s, conn, 5)
	if err != nil {
		fmt.Printf("CONNECTION FAILED -> %v\n", err.Error())
	}
	fmt.Printf("CONNECTION SUCCESS -> %v\n", peerConn.Info.Pid.String())
	peerConn.torr.NewPeer <- peerConn
}
