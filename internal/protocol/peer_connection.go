package protocol

import (
	"GoBit/internal/utils"
	"bytes"
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
	err := sendHandshake(c.conn, infohash, clientPid)
	if err != nil {
		return err
	}

	pid, ih, err := receiveHandshake(c.conn)
	if err != nil {
		return err
	}

	if ih != infohash {
		return Peer_handshake_infohash_err
	}

	c.Pid = pid
	return nil
}

func (c *PeerConnection) handshakeIncomingPeer(torrents map[[20]byte]*Torrent, clientPid PeerID) (*Torrent, error) {
	pid, ih, err := receiveHandshake(c.conn)
	if err != nil {
		return nil, Peer_bad_handshake_err
	}

	torrent, ok := torrents[ih]
	if !ok {
		return nil, Peer_bad_handshake_err
	}

	err = sendHandshake(c.conn, ih, clientPid)

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
				p.torrent.RemoveActiveConnection(p)
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
		panic("bad message sent")
	}
	err = utils.WriteFull(p.conn, content)
	if err != nil {
		p.cancel(fmt.Errorf("%w (%w)", Peer_write_err, err))
	}
}

func sendHandshake(conn net.Conn, ih, pid [20]byte) error {
	handshake := []byte{
		0x13,
		'B', 'i', 't',
		'T', 'o', 'r', 'r', 'e', 'n', 't',
		' ',
		'p', 'r', 'o', 't', 'o', 'c', 'o', 'l',
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
	}
	handshake = append(handshake, ih[:]...)
	handshake = append(handshake, pid[:]...)

	err := utils.WriteFull(conn, handshake)
	if err != nil {
		return err
	}

	return nil
}

func receiveHandshake(conn net.Conn) ([20]byte, [20]byte, error) {
	handshake := []byte{
		0x13,
		'B', 'i', 't',
		'T', 'o', 'r', 'r', 'e', 'n', 't',
		' ',
		'p', 'r', 'o', 't', 'o', 'c', 'o', 'l',
	}

	buf := make([]byte, 68)

	conn.SetDeadline(time.Now().Add(time.Second * 10))
	_, err := io.ReadFull(conn, buf)
	conn.SetDeadline(time.Time{})

	pid := [20]byte{}
	ih := [20]byte{}

	if err != nil {
		return pid, ih, fmt.Errorf("%v (%v)", Peer_handshake_err, err)
	}

	if !bytes.Equal(buf[:20], handshake[:20]) {
		return pid, ih, Peer_bad_handshake_err
	}

	pid = ([20]byte)(buf[48:])
	ih = ([20]byte)(buf[28:48])

	return pid, ih, nil
}

func (p *PeerConnection) handleMessage(mess peerMessage) {
	switch mess.Kind {
	case KeepAlive:
		{
			p.keepAlivePeer.Reset(p.peerTimeout)
		}
	case Choke:
		if mess.Payload != nil {
			p.cancel(Peer_bad_message_err)
			return
		}
		p.torrent.ReceiveMessage(mess)
	case Unchoke:
		if mess.Payload != nil {
			p.cancel(Peer_bad_message_err)
			return
		}
		p.torrent.ReceiveMessage(mess)
	case Interested:
		if mess.Payload != nil {
			p.cancel(Peer_bad_message_err)
			return
		}
		p.torrent.ReceiveMessage(mess)
	case Uninterested:
		if mess.Payload != nil {
			p.cancel(Peer_bad_message_err)
			return
		}
		p.torrent.ReceiveMessage(mess)
	case Have:
		if len(mess.Payload) != 4 {
			p.cancel(Peer_bad_message_err)
			return
		}
		p.torrent.ReceiveMessage(mess)
	case Bitfield:
		if p.bitfieldSent {
			p.cancel(Peer_double_bitfield)
			return
		}
		p.torrent.ReceiveMessage(mess)
	case Request:
		if len(mess.Payload) != 13 {
			p.cancel(Peer_bad_message_err)
			return
		}
		p.torrent.ReceiveMessage(mess)
	case Piece:
		if uint32(len(mess.Payload)) != 9+p.torrent.Picker.blockLength {
			p.cancel(Peer_bad_message_err)
			return
		}
		p.torrent.ReceiveMessage(mess)
	case Cancel:
		if len(mess.Payload) != 13 {
			p.cancel(Peer_bad_message_err)
			return
		}
		p.torrent.ReceiveMessage(mess)
	default:
		p.cancel(Peer_unrecognized_mess_err)
	}
}
