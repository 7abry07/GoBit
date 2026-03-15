package protocol

import (
	"GoBit/internal/utils"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

type PeerConnection struct {
	Pid    PeerID
	ctx    context.Context
	cancel context.CancelCauseFunc

	keepAlivePeer *time.Timer
	keepAliveFreq time.Duration
	peerTimeout   time.Duration

	conn    net.Conn
	torrent *Torrent

	in    chan peerMessage
	out   chan peerMessage
	queue chan PeerRequest

	Pieces *utils.Bitfield

	IsChoked      bool
	IsInteresting bool
	AmChoked      bool
	AmInteresting bool

	bitfieldSent bool
}

func newPeerConnection(conn net.Conn) *PeerConnection {
	c := PeerConnection{}
	c.AmChoked = true
	c.IsChoked = true
	c.AmInteresting = false
	c.IsInteresting = false
	c.bitfieldSent = false

	c.keepAliveFreq = time.Minute
	c.peerTimeout = time.Minute * 3
	c.out = make(chan peerMessage)
	c.in = make(chan peerMessage)
	c.queue = make(chan PeerRequest)

	c.conn = conn
	c.Pid = PeerID{}

	c.torrent = nil
	c.Pieces = nil

	return &c
}

func (c *PeerConnection) handshakePeer(infohash [20]byte, clientPid PeerID) error {
	err := utils.SendHandshake(c.conn, infohash, clientPid)
	if err != nil {
		return err
	}

	pid, ih, ok := utils.ReceiveHandshake(c.conn)
	if !ok || ih != infohash {
		return Peer_bad_handshake_err
	}

	c.Pid = pid
	return nil
}

func (c *PeerConnection) handshakeIncomingPeer(torrents map[[20]byte]*Torrent, clientPid PeerID) (*Torrent, error) {
	pid, ih, ok := utils.ReceiveHandshake(c.conn)
	if !ok {
		return nil, Peer_bad_handshake_err
	}

	torrent, ok := torrents[ih]
	if !ok {
		return nil, Peer_bad_handshake_err
	}

	err := utils.SendHandshake(c.conn, ih, clientPid)

	if err != nil {
		return nil, err
	}

	c.Pid = pid
	return torrent, nil
}

func (p *PeerConnection) start() {
	p.keepAlivePeer = time.NewTimer(p.peerTimeout)
	go p.loop()
}

func (c *PeerConnection) attachTorrent(t *Torrent) {
	c.ctx, c.cancel = context.WithCancelCause(t.ctx)
	c.Pieces = utils.NewBitfield(uint32(len(t.Info.Pieces) / 20))
	c.torrent = t
}

func (p *PeerConnection) loop() {
	go p.receiveLoop()

	for {
		select {
		case <-p.ctx.Done():
			{
				fmt.Printf(
					"%v DISCONNECTED BECAUSE: %v\n",
					p.Pid.String(), context.Cause(p.ctx).Error())
				p.conn.Close()
				return
			}
		case mess := <-p.in:
			p.keepAlivePeer.Reset(p.peerTimeout)
			go p.handleMessage(mess)
		case mess := <-p.out:
			go p.send(mess)
		case <-p.keepAlivePeer.C:
			p.cancel(Peer_timeout)
		}
	}
}

func (p *PeerConnection) receiveLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			lenBuf := make([]byte, 4)

			_, err := io.ReadFull(p.conn, lenBuf)
			if err != nil {
				p.cancel(fmt.Errorf("%w (%w)", Peer_read_err, err))
				return
			}

			length := binary.BigEndian.Uint32(lenBuf)
			messBuf := make([]byte, length)

			_, err = io.ReadFull(p.conn, messBuf)

			if err != nil {
				p.cancel(fmt.Errorf("%w (%w)", Peer_read_err, err))
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

func (p *PeerConnection) KeepAlive() {
	ka := peerMessage{
		Peer:    p,
		Kind:    KeepAlive,
		Payload: nil,
	}

	p.out <- ka
}

func (p *PeerConnection) SendBlock(res PeerBlockResponse) {
	mess := peerMessage{}
	mess.Kind = Piece
	binary.LittleEndian.AppendUint32(mess.Payload, res.Idx)
	binary.LittleEndian.AppendUint32(mess.Payload, res.Begin)
	mess.Payload = append(mess.Payload, res.Block...)
	p.out <- mess
}

func (p *PeerConnection) SendHave(idx uint32) {
	mess := peerMessage{}
	mess.Kind = Have
	binary.LittleEndian.AppendUint32(mess.Payload, idx)
	p.out <- mess
}

func (p *PeerConnection) SendBitfield(bitfield *utils.Bitfield) {
	mess := peerMessage{}
	mess.Kind = Bitfield
	mess.Payload = bitfield.Data()
	p.out <- mess
}

func (p *PeerConnection) SendInterested(v bool) {
	mess := peerMessage{}
	if v {
		mess.Kind = Interested
	} else {
		mess.Kind = Interested
	}
	mess.Payload = nil
	p.out <- mess
}

func (p *PeerConnection) SendChoked(v bool) {
	mess := peerMessage{}
	if v {
		mess.Kind = Choke
	} else {
		mess.Kind = Unchoke
	}
	mess.Payload = nil
	p.out <- mess
}

func (p *PeerConnection) CancelRequest(req PeerRequest) {
	mess := peerMessage{}
	mess.Kind = Cancel
	binary.LittleEndian.AppendUint32(mess.Payload, req.Idx)
	binary.LittleEndian.AppendUint32(mess.Payload, req.Begin)
	binary.LittleEndian.AppendUint32(mess.Payload, req.Length)
	p.out <- mess
}

func (p *PeerConnection) send(mess peerMessage) {
	content, err := mess.ToNetwork()
	if err != nil {
		p.cancel(Peer_malformed_mess_sent)
	}
	err = utils.WriteFull(p.conn, content)
	if err != nil {
		p.cancel(fmt.Errorf("%w (%w)", Peer_write_err, err))
	}
}

func (p *PeerConnection) handleMessage(mess peerMessage) {
	switch mess.Kind {
	case KeepAlive:
		{
			fmt.Printf("KEEP ALIVE RECEIVED -> %v\n", p.Pid.String())
			p.keepAlivePeer.Reset(p.peerTimeout)
		}
	case Choke:
		{
			if mess.Payload != nil {
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.AmChoked = true
		}
	case Unchoke:
		{
			if mess.Payload != nil {
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.AmChoked = false
		}
	case Interested:
		{
			if mess.Payload != nil {
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.AmInteresting = true
		}
	case Uninterested:
		{
			if mess.Payload != nil {
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.AmInteresting = false
		}
	case Have:
		{
			p.torrent.ReceiveMessage(mess)
		}
	case Bitfield:
		{
			if p.bitfieldSent {
				p.cancel(Peer_double_bitfield)
				return
			}
			p.bitfieldSent = true
			p.torrent.ReceiveMessage(mess)
		}
	default:
		{
			p.torrent.ReceiveMessage(mess)
		}
	}
}
