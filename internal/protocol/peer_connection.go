package protocol

import (
	"GoBit/internal/utils"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"time"
)

type peerConnection struct {
	ctx           context.Context
	cancel        context.CancelCauseFunc
	keepAliveSelf *time.Ticker
	keepAlivePeer *time.Timer
	keepAliveFreq time.Duration
	peerTimeout   time.Duration

	conn net.Conn
	torr *Torrent

	in  chan peerMessage
	out chan peerMessage

	requestQueue chan PeerRequest

	pieces *utils.Bitfield

	Info          Peer
	IsChoked      bool
	IsInteresting bool
	AmChoked      bool
	AmInteresting bool
	bitfieldSent  bool
}

func newPeerConnection(t *Torrent, p Peer, pid PeerID, queueSize int) (*peerConnection, error) {
	peerConn := peerConnection{}
	peerConn.ctx, peerConn.cancel = context.WithCancelCause(t.ctx)
	peerConn.Info = p
	peerConn.AmChoked = true
	peerConn.IsChoked = true
	peerConn.AmInteresting = false
	peerConn.IsInteresting = false
	peerConn.bitfieldSent = false
	peerConn.out = make(chan peerMessage)
	peerConn.in = make(chan peerMessage)
	peerConn.requestQueue = make(chan PeerRequest, queueSize)
	peerConn.pieces = utils.NewBitfield(uint32(len(t.Info.Pieces) / 20))
	peerConn.keepAliveFreq = time.Minute
	peerConn.peerTimeout = time.Minute * 3
	peerConn.keepAliveSelf = time.NewTicker(peerConn.keepAliveFreq)
	peerConn.keepAlivePeer = time.NewTimer(peerConn.peerTimeout)
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
		return nil, Peer_bad_handshake_err
	}

	peerConn.Info.Pid = pid
	peerConn.conn = conn

	return &peerConn, nil
}

func newIncomingPeerConnection(s *Session, conn net.Conn, queueSize int) (*peerConnection, error) {
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
	peerConn.ctx, peerConn.cancel = context.WithCancelCause(torrent.ctx)
	peerConn.Info = peerInfo
	peerConn.conn = conn
	peerConn.bitfieldSent = false
	peerConn.pieces = utils.NewBitfield(uint32(len(torrent.Info.Pieces) / 20))
	peerConn.AmChoked = true
	peerConn.IsChoked = true
	peerConn.AmInteresting = false
	peerConn.IsInteresting = false
	peerConn.requestQueue = make(chan PeerRequest, queueSize)
	peerConn.out = make(chan peerMessage)
	peerConn.in = make(chan peerMessage)
	peerConn.keepAliveSelf = time.NewTicker(time.Minute)
	peerConn.keepAlivePeer = time.NewTimer(time.Minute * 3)
	peerConn.torr = torrent

	return &peerConn, nil
}

func (p *peerConnection) start() {
	go p.loop()
}

func (p *peerConnection) readLoop() {
	go p.receiveLoop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case mess := <-p.in:
			go p.handleMessage(mess)
		}
	}
}
func (p *peerConnection) writeLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case mess := <-p.out:
			go p.send(mess)
		}
	}
}

func (p *peerConnection) loop() {
	go p.readLoop()
	go p.writeLoop()

	for {
		select {
		case <-p.ctx.Done():
			{
				fmt.Printf("PEER REMOVED -> %v REASON -> %v\n", string(p.Info.Pid.String()), context.Cause(p.ctx))
				p.torr.RemovePeer(p)
				p.conn.Close()
				return
			}
		case req := <-p.requestQueue:
			{
				mess := peerMessage{}
				mess.Kind = Request
				binary.LittleEndian.AppendUint32(mess.Payload, req.Idx)
				binary.LittleEndian.AppendUint32(mess.Payload, req.Begin)
				binary.LittleEndian.AppendUint32(mess.Payload, req.Length)
				p.out <- mess
			}
		case <-p.keepAliveSelf.C:
			{
				mess := peerMessage{}
				mess.Kind = KeepAlive
				mess.Payload = nil
				p.out <- mess
				p.keepAliveSelf.Reset(p.keepAliveFreq)
				fmt.Printf("KEEP ALIVE SENT -> %v\n", p.Info.Pid.String())
			}
		case <-p.keepAlivePeer.C:
			{
				fmt.Printf("PEER ERROR TIMEOUT -> %v\n", p.Info.Pid.String())
				p.cancel(Peer_timeout)
			}
		}
	}
}

func (p *peerConnection) receiveLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			lenBuf := make([]byte, 4)

			_, err := io.ReadFull(p.conn, lenBuf)
			if err != nil {
				p.cancel(errors.Join(Peer_read_err, err))
				return
			}

			length := binary.BigEndian.Uint32(lenBuf)
			messBuf := make([]byte, length)

			_, err = io.ReadFull(p.conn, messBuf)

			if err != nil {
				p.cancel(errors.Join(Peer_read_err, err))
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
}

func (p *peerConnection) HasPiece(idx uint32) bool {
	return p.pieces.IsSet(idx)
}

func (p *peerConnection) SendBlock(res PeerBlockResponse) {
	mess := peerMessage{}
	mess.Kind = Piece
	binary.LittleEndian.AppendUint32(mess.Payload, res.Idx)
	binary.LittleEndian.AppendUint32(mess.Payload, res.Begin)
	mess.Payload = append(mess.Payload, res.Block...)
	p.out <- mess
}

func (p *peerConnection) SendHave(idx uint32) {
	mess := peerMessage{}
	mess.Kind = Have
	binary.LittleEndian.AppendUint32(mess.Payload, idx)
	p.out <- mess
}

func (p *peerConnection) SendBitfield(bitfield *utils.Bitfield) {
	mess := peerMessage{}
	mess.Kind = Bitfield
	mess.Payload = bitfield.Data()
	p.out <- mess
}

func (p *peerConnection) SetInterested(v bool) {
	mess := peerMessage{}
	if v {
		mess.Kind = Interested
	} else {
		mess.Kind = Interested
	}
	mess.Payload = nil
	p.out <- mess
}

func (p *peerConnection) SetChoked(v bool) {
	mess := peerMessage{}
	if v {
		mess.Kind = Choke
	} else {
		mess.Kind = Unchoke
	}
	mess.Payload = nil
	p.out <- mess
}

func (p *peerConnection) CancelRequest(req PeerRequest) {
	mess := peerMessage{}
	mess.Kind = Cancel
	binary.LittleEndian.AppendUint32(mess.Payload, req.Idx)
	binary.LittleEndian.AppendUint32(mess.Payload, req.Begin)
	binary.LittleEndian.AppendUint32(mess.Payload, req.Length)
	p.out <- mess
}

func (p *peerConnection) QueueRequest(req PeerRequest) {
	p.requestQueue <- req
}

func (p *peerConnection) send(mess peerMessage) {
	content, err := mess.ToNetwork()
	if err != nil {
		p.cancel(Peer_malformed_mess_sent)
	}
	err = utils.WriteFull(p.conn, content)
	if err != nil {
		p.cancel(errors.Join(Peer_write_err, err))
	}
}

func (p *peerConnection) handleMessage(mess peerMessage) {
	switch mess.Kind {
	case KeepAlive:
		{
			fmt.Printf("KEEP ALIVE RECEIVED -> %v\n", mess.Peer.Info.Pid.String())
			p.keepAlivePeer.Reset(p.peerTimeout)
		}
	case Choke:
		{
			if mess.Payload != nil {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "non nil payload")
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.AmChoked = true
		}
	case Unchoke:
		{
			if mess.Payload != nil {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "non nil payload")
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.AmChoked = false
		}
	case Interested:
		{
			if mess.Payload != nil {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "non nil payload")
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.AmInteresting = true
		}
	case Uninterested:
		{
			if mess.Payload != nil {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "non nil payload")
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.AmInteresting = false
		}
	case Have:
		{
			idx := binary.LittleEndian.Uint32(mess.Payload)
			ok := p.pieces.Set(idx, true)
			if !ok {
				fmt.Printf("PEER %v ERR -> %v %v:%v\n", string(p.Info.Pid.String()), "out of bounds index", idx/8, p.pieces.Length())
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.torr.ReceiveMessage(mess)
		}
	case Bitfield:
		{
			if p.bitfieldSent {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "double bitfield sent")
				p.cancel(Peer_double_bitfield)
				return
			}
			p.bitfieldSent = true
			ok := p.pieces.SetBitfield(mess.Payload)
			if !ok {
				fmt.Printf("PEER %v ERR -> %v\n", string(p.Info.Pid.String()), "bitfield length mismatch")
			}
			p.torr.ReceiveMessage(mess)
		}
	default:
		{
			p.torr.ReceiveMessage(mess)
		}
	}
}
