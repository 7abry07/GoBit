package protocol

import (
	"GoBit/internal/utils"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/bits-and-blooms/bitset"
	"io"
	"net"
	"time"
)

type PeerConnection struct {
	Pid    PeerID
	ctx    context.Context
	cancel context.CancelCauseFunc

	Peer *Peer

	keepAlivePeer *time.Timer
	peerTimeout   time.Duration

	conn    net.Conn
	torrent *Torrent

	in  chan peerMessage
	out chan peerMessage

	bitfieldSent bool
}

func newPeerConnection(conn net.Conn) *PeerConnection {
	c := PeerConnection{}

	c.peerTimeout = time.Minute * 3

	c.in = make(chan peerMessage)
	c.out = make(chan peerMessage, 1024)

	c.conn = conn
	c.Pid = PeerID{}

	c.torrent = nil

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

func (p *PeerConnection) loop() {
	go p.readLoop()
	go p.writeLoop()
	go p.receiveLoop()

	for {
		select {
		case <-p.ctx.Done():
			{
				p.torrent.SignalEvent(PeerDisconnected{Sender: p.Pid, Cause: context.Cause(p.ctx)})
				p.conn.Close()
				return
			}
		}
	}
}

func (p *PeerConnection) readLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case mess := <-p.in:
			p.keepAlivePeer.Reset(p.peerTimeout)
			p.handleMessage(mess)
		}
	}
}

func (p *PeerConnection) writeLoop() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case mess := <-p.out:
			p.send(mess)
		}
	}
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
	c.torrent = t
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

func (p *PeerConnection) SendKeepAlive() {
	ka := peerMessage{
		Peer:    p,
		Kind:    KeepAlive,
		Payload: nil,
	}

	p.out <- ka
}

func (p *PeerConnection) SendInterested(v bool) {
	mess := peerMessage{}
	if v {
		mess.Kind = Interested
	} else {
		mess.Kind = Uninterested
	}
	mess.Payload = nil
	mess.Peer = p
	p.out <- mess
}

func (p *PeerConnection) SendChoked(v bool) {
	mess := peerMessage{}
	if v {
		mess.Kind = Choke
	} else {
		mess.Kind = Unchoke
	}
	mess.Peer = p
	mess.Payload = nil
	p.out <- mess
}

func (p *PeerConnection) SendBlock(idx, begin uint32, block []byte) {
	mess := peerMessage{}
	mess.Kind = Piece
	mess.Peer = p
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, idx)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, begin)
	mess.Payload = append(mess.Payload, block...)
	p.out <- mess
}

func (p *PeerConnection) SendHave(idx uint32) {
	mess := peerMessage{}
	mess.Kind = Have
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, idx)
	mess.Peer = p
	p.out <- mess
}

func (p *PeerConnection) SendBitfield(bf bitset.BitSet) {
	mess := peerMessage{}
	mess.Kind = Bitfield

	raw := utils.BitSetToBytes(bf)
	mess.Payload = raw
	p.out <- mess
}

func (p *PeerConnection) SendRequest(idx, begin, length uint32) {
	mess := peerMessage{}
	mess.Kind = Request
	mess.Peer = p
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, idx)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, begin)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, length)
	p.out <- mess
}

func (p *PeerConnection) SendCancel(idx, begin, length uint32) {
	mess := peerMessage{}
	mess.Kind = Cancel
	mess.Peer = p
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, idx)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, begin)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, length)
	p.out <- mess
}

func (p *PeerConnection) send(mess peerMessage) {
	content, err := mess.ToNetwork()
	if err != nil {
		panic("bad message sent")
	}

	// if len(content) != 4 && content[4] != 0x05 {
	// 	fmt.Printf("out -> [%x]\n", content)
	// }

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
			p.cancel(fmt.Errorf("%v (choke)", Peer_bad_message_err))
			return
		}
		p.torrent.SignalEvent(PeerChoke{p.Pid, true})
	case Unchoke:
		if mess.Payload != nil {
			p.cancel(Peer_bad_message_err)
			p.cancel(fmt.Errorf("%v (unchoke)", Peer_bad_message_err))
			return
		}
		p.torrent.SignalEvent(PeerChoke{p.Pid, false})
	case Interested:
		if mess.Payload != nil {
			p.cancel(fmt.Errorf("%v (interested)", Peer_bad_message_err))
			return
		}
		p.torrent.SignalEvent(PeerInterested{p.Pid, true})
	case Uninterested:
		if mess.Payload != nil {
			p.cancel(fmt.Errorf("%v (uninterested)", Peer_bad_message_err))
			return
		}
		p.torrent.SignalEvent(PeerInterested{p.Pid, false})
	case Have:
		if len(mess.Payload) != 4 {
			p.cancel(fmt.Errorf("%v (have)", Peer_bad_message_err))
			return
		}

		idx := binary.LittleEndian.Uint32(mess.Payload)
		p.torrent.SignalEvent(PeerHave{p.Pid, idx})
	case Bitfield:
		if p.bitfieldSent {
			p.cancel(Peer_double_bitfield)
			return
		}
		p.bitfieldSent = true

		bf := utils.BytesToBitSet(mess.Payload, uint(p.torrent.Picker.pieceCount))
		p.torrent.SignalEvent(PeerBitfield{p.Pid, bf})
	case Request:
		if len(mess.Payload) != 12 {
			p.cancel(fmt.Errorf("%v (request)", Peer_bad_message_err))
			return
		}

		idx := binary.LittleEndian.Uint32(mess.Payload[:4])
		begin := binary.LittleEndian.Uint32(mess.Payload[4:8])
		length := binary.LittleEndian.Uint32(mess.Payload[8:])

		if length > p.torrent.Info.BlockSize {
			p.cancel(Peer_request_too_large)
		}
		p.torrent.SignalEvent(PeerRequest{p.Pid, idx, begin, length})
	case Piece:
		if uint32(len(mess.Payload)) < 8 {
			p.cancel(fmt.Errorf("%v (piece)", Peer_bad_message_err))
			return
		}
		idx := binary.LittleEndian.Uint32(mess.Payload[:4])
		begin := binary.LittleEndian.Uint32(mess.Payload[4:8])
		block := mess.Payload[8:]
		p.torrent.SignalEvent(PeerPiece{p.Pid, idx, begin, block})
	case Cancel:
		if len(mess.Payload) != 12 {
			p.cancel(fmt.Errorf("%v (cancel)", Peer_bad_message_err))
			return
		}
		idx := binary.LittleEndian.Uint32(mess.Payload[:4])
		begin := binary.LittleEndian.Uint32(mess.Payload[4:8])
		length := binary.LittleEndian.Uint32(mess.Payload[8:])
		p.torrent.SignalEvent(PeerCancel{p.Pid, idx, begin, length})
	default:
		p.cancel(Peer_unrecognized_mess_err)
	}
}
