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

type ActivePeer struct {
	Conn  *PeerConnection
	State *ActivePeerState
}

type ActivePeerState struct {
	LastTickTime     time.Time
	TotalDownloaded  int
	TotalUploaded    int
	LastTickDownload int
	LastTickUpload   int
	DownloadRate     float64
	UploadRate       float64

	Pieces *bitset.BitSet

	PendingRequests []PeerRequest

	IsChoked      bool
	IsInteresting bool
	AmChoked      bool
	AmInteresting bool
	IsOptimistic  bool
}

type PeerConnection struct {
	Pid    PeerID
	ctx    context.Context
	cancel context.CancelCauseFunc

	Peer *Peer

	keepAlivePeer *time.Timer
	peerTimeout   time.Duration

	conn    net.Conn
	torrent *Torrent

	in chan peerMessage

	bitfieldSent bool
}

func newPeerConnection(conn net.Conn) *PeerConnection {
	c := PeerConnection{}

	c.peerTimeout = time.Minute * 3
	c.in = make(chan peerMessage)

	c.conn = conn
	c.Pid = PeerID{}

	c.torrent = nil
	// c.Pieces = nil

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
	// go p.writeLoop()
	go p.receiveLoop()

	for {
		select {
		case <-p.ctx.Done():
			{
				disconnectEv := PeerDisconnectedEv{
					Sender: p.Pid,
					Cause:  context.Cause(p.ctx),
				}
				p.torrent.SignalEvent(disconnectEv)
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

// func (p *PeerConnection) writeLoop() {
// 	for {
// 		select {
// 		case <-p.ctx.Done():
// 			return
// 		case mess := <-p.out:
// 			go p.send(mess)
// 		}
// 	}
// }

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
	// c.Pieces = utils.NewBitfield(uint32(len(t.Info.Pieces) / 20))
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

func (p *PeerConnection) KeepAlive() {
	ka := peerMessage{
		Peer:    p,
		Kind:    KeepAlive,
		Payload: nil,
	}

	go p.send(ka)
}

func (p *PeerConnection) SendBlock(idx, begin uint32, block []byte) {
	mess := peerMessage{}
	mess.Kind = Piece
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, idx)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, begin)
	mess.Payload = append(mess.Payload, block...)
	go p.send(mess)
}

func (p *PeerConnection) SendHave(idx uint32) {
	mess := peerMessage{}
	mess.Kind = Have
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, idx)
	go p.send(mess)
}

func (p *PeerConnection) SendBitfield(bf bitset.BitSet) {
	mess := peerMessage{}
	mess.Kind = Bitfield

	words := bf.Words()
	raw := make([]byte, 8*len(words))
	for i, w := range words {
		binary.LittleEndian.PutUint64(raw[i*8:], w)
	}
	mess.Payload = raw
	go p.send(mess)
}

func (p *PeerConnection) SendInterested(v bool) {
	mess := peerMessage{}
	if v {
		mess.Kind = Interested
	} else {
		mess.Kind = Uninterested
	}
	mess.Payload = nil
	go p.send(mess)
}

func (p *PeerConnection) SendChoked(v bool) {
	mess := peerMessage{}
	if v {
		mess.Kind = Choke
	} else {
		mess.Kind = Unchoke
	}
	mess.Payload = nil
	go p.send(mess)
}

func (p *PeerConnection) SendRequest(req PeerRequest) {
	mess := peerMessage{}
	mess.Kind = Request
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, req.Idx)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, req.Begin)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, req.Length)
	go p.send(mess)
}

func (p *PeerConnection) CancelRequest(req PeerRequest) {
	mess := peerMessage{}
	mess.Kind = Cancel
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, req.Idx)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, req.Begin)
	mess.Payload = binary.LittleEndian.AppendUint32(mess.Payload, req.Length)
	go p.send(mess)
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
			p.cancel(fmt.Errorf("%v (choke)", Peer_bad_message_err))
			return
		}
		chokeEv := PeerChokeEv{
			Sender: p.Pid,
			Value:  true,
		}
		p.torrent.SignalEvent(chokeEv)
	case Unchoke:
		if mess.Payload != nil {
			p.cancel(Peer_bad_message_err)
			p.cancel(fmt.Errorf("%v (unchoke)", Peer_bad_message_err))
			return
		}
		unchokeEv := PeerChokeEv{
			Sender: p.Pid,
			Value:  false,
		}
		p.torrent.SignalEvent(unchokeEv)
	case Interested:
		if mess.Payload != nil {
			p.cancel(fmt.Errorf("%v (interested)", Peer_bad_message_err))
			return
		}
		interestedEv := PeerInterestedEv{
			Sender: p.Pid,
			Value:  true,
		}
		p.torrent.SignalEvent(interestedEv)
	case Uninterested:
		if mess.Payload != nil {
			p.cancel(fmt.Errorf("%v (uninterested)", Peer_bad_message_err))
			return
		}
		uninterestedEv := PeerInterestedEv{
			Sender: p.Pid,
			Value:  false,
		}
		p.torrent.SignalEvent(uninterestedEv)
	case Have:
		if len(mess.Payload) != 4 {
			p.cancel(fmt.Errorf("%v (have)", Peer_bad_message_err))
			return
		}

		idx := binary.LittleEndian.Uint32(mess.Payload)

		haveEv := PeerHaveEv{
			Sender: p.Pid,
			Idx:    idx,
		}

		p.torrent.SignalEvent(haveEv)
	case Bitfield:
		if p.bitfieldSent {
			p.cancel(Peer_double_bitfield)
			return
		}
		p.bitfieldSent = true

		bf := utils.BytesToBitSet(mess.Payload, p.torrent.Picker.pieceCount)

		bfEv := PeerBitfieldEv{
			Sender:   p.Pid,
			Bitfield: bf,
		}

		p.torrent.SignalEvent(bfEv)
	case Request:
		if len(mess.Payload) != 12 {
			p.cancel(fmt.Errorf("%v (request)", Peer_bad_message_err))
			return
		}

		idx := binary.LittleEndian.Uint32(mess.Payload[:4])
		begin := binary.LittleEndian.Uint32(mess.Payload[4:8])
		length := binary.LittleEndian.Uint32(mess.Payload[8:])

		reqEv := PeerRequestEv{
			Sender: p.Pid,
			Idx:    idx,
			Begin:  begin,
			Length: length,
		}

		p.torrent.SignalEvent(reqEv)
	case Piece:
		if uint32(len(mess.Payload)) != 8+p.torrent.Info.BlockLength {
			p.cancel(fmt.Errorf("%v (piece)", Peer_bad_message_err))
			return
		}

		idx := binary.LittleEndian.Uint32(mess.Payload[:4])
		begin := binary.LittleEndian.Uint32(mess.Payload[4:8])
		block := mess.Payload[8:]

		pieceEv := PeerPieceEv{
			Sender: p.Pid,
			Idx:    idx,
			Begin:  begin,
			Block:  block,
		}

		p.torrent.SignalEvent(pieceEv)
	case Cancel:
		if len(mess.Payload) != 12 {
			p.cancel(fmt.Errorf("%v (cancel)", Peer_bad_message_err))
			return
		}

		idx := binary.LittleEndian.Uint32(mess.Payload[:4])
		begin := binary.LittleEndian.Uint32(mess.Payload[4:8])
		length := binary.LittleEndian.Uint32(mess.Payload[8:])

		cancelEv := PeerCancelEv{
			Sender: p.Pid,
			Idx:    idx,
			Begin:  begin,
			Length: length,
		}

		p.torrent.SignalEvent(cancelEv)
	default:
		p.cancel(Peer_unrecognized_mess_err)
	}
}
