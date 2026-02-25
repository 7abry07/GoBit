package protocol

import (
	"GoBit/internal/utils"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"
)

type peerConnection struct {
	ctx    context.Context
	cancel context.CancelFunc

	conn net.Conn
	torr *Torrent

	in  chan PeerMessage
	Out chan PeerMessage

	pieces []uint8

	Info          Peer
	IsChoked      bool
	IsInteresting bool
	AmChoked      bool
	AmInteresting bool
	bitfieldSent  bool
}

func newPeerConnection(t *Torrent, p Peer, pid PeerID) (*peerConnection, error) {
	peerConn := peerConnection{}
	peerConn.ctx, peerConn.cancel = context.WithCancel(t.ctx)
	peerConn.Info = p
	peerConn.AmChoked = true
	peerConn.IsChoked = true
	peerConn.AmInteresting = false
	peerConn.IsInteresting = false
	peerConn.bitfieldSent = false
	peerConn.Out = make(chan PeerMessage)
	peerConn.in = make(chan PeerMessage)
	peerConn.pieces = make([]uint8, len(t.Info.Pieces)/20)
	peerConn.torr = t

	conn, err := net.DialTimeout("tcp", p.IpPort.String(), time.Second*3)
	if err != nil {
		return nil, err
	}

	handshakeReq, err := utils.SendHandshake(conn, t.Info.InfoHash, pid)
	if err != nil {
		conn.Close()
		return nil, err
	}

	pid, ok := utils.ReceiveHandshake(conn, handshakeReq)
	if !ok {
		conn.Close()
		return nil, err
	}

	peerConn.Info.Pid = pid
	peerConn.conn = conn

	go peerConn.loop()

	return &peerConn, nil
}

func newIncomingPeerConnection(s *Session, conn net.Conn) (*peerConnection, error) {
	buf := make([]byte, 68)
	bytesRead, err := io.ReadFull(conn, buf)
	if err != nil || bytesRead != 68 {
		conn.Close()
		return nil, err
	}
	if string(buf[0:20]) != "\x13BitTorrent protocol" {
		conn.Close()
		return nil, err
	}

	infohash := [20]byte(buf[29:49])
	pid := [20]byte(buf[49:])

	torrent, found := s.Torrents[infohash]
	if !found {
		conn.Close()
		return nil, err
	}

	_, err = utils.SendHandshake(conn, infohash, s.PeerID)
	if err != nil {
		conn.Close()
		return nil, err
	}

	endpoint, err := netip.ParseAddrPort(conn.LocalAddr().String())
	if err != nil {
		panic(err)
	}
	peerInfo := Peer{Pid: pid, IpPort: endpoint}

	peerConn := peerConnection{}
	peerConn.ctx, peerConn.cancel = context.WithCancel(torrent.ctx)
	peerConn.Info = peerInfo
	peerConn.conn = conn
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

	return &peerConn, nil
}

func (p *peerConnection) loop() {
	go p.receiveLoop()

	for {
		select {
		case <-p.ctx.Done():
			{
				fmt.Printf("PEER REMOVED -> %v\n", string(p.Info.Pid.String()))
				p.conn.Close()
				return
			}
		case mess := <-p.Out:
			{
				err := p.send(mess)
				if err != nil {
					p.cancel()
				}
			}
		case mess := <-p.in:
			{
				go p.handleMessage(mess)
			}
		}
	}
}

func (p *peerConnection) send(mess PeerMessage) error {
	content, err := mess.ToNetwork()
	if err != nil {
		return err
	}
	return utils.WriteFull(p.conn, content)
}

func (p *peerConnection) receiveLoop() {
	for {
		lenBuf := make([]byte, 4)

		bytesRead, err := io.ReadFull(p.conn, lenBuf)
		if err != nil || bytesRead < 4 {
			p.cancel()
			return
		}

		length := binary.BigEndian.Uint32(lenBuf)
		messBuf := make([]byte, length)

		bytesRead, err = io.ReadFull(p.conn, messBuf)

		if err != nil || bytesRead < int(length) {
			p.cancel()
			return
		}

		input := []byte{}
		input = append(input, lenBuf...)
		input = append(input, messBuf...)

		mess, err := fromNetwork(input)
		mess.Peer = p
		p.in <- mess
	}
}

func (p *peerConnection) handleMessage(mess PeerMessage) {
	switch mess.Kind {
	case Choke:
		{
			if mess.Payload != nil {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "non nil payload")
				p.cancel()
				return
			}
			p.AmChoked = true
		}
	case Unchoke:
		{
			if mess.Payload != nil {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "non nil payload")
				p.cancel()
				return
			}
			p.AmChoked = false
		}
	case Interested:
		{
			if mess.Payload != nil {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "non nil payload")
				p.cancel()
				return
			}
			p.AmInteresting = true
		}
	case Uninterested:
		{
			if mess.Payload != nil {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "non nil payload")
				p.cancel()
				return
			}
			p.AmInteresting = false
		}
	case Have:
		{
			idx := binary.LittleEndian.Uint32(mess.Payload)
			byteIdx := idx / 8
			if byteIdx >= uint32(len(p.pieces)) {
				fmt.Printf("PEER %v ERR -> %v %v:%v\n", string(p.Info.Pid.String()), "out of bounds index", byteIdx, uint32(len(p.pieces)))
				p.cancel()
				return
			}
			bitIdx := 7 - (idx % 8)
			p.pieces[byteIdx] |= 1 << bitIdx
			p.torr.PeerInbox <- mess
		}
	case Bitfield:
		{
			if p.bitfieldSent {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "double bitfield sent")
				p.cancel()
				return
			}
			p.bitfieldSent = true
			p.pieces = []byte{}
			p.pieces = append(p.pieces, mess.Payload...)
		}
	default:
		{
			p.torr.PeerInbox <- mess
		}
	}
}
