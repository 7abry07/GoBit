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

type PeerEndpoint struct {
	Pid    *PeerID
	IpPort netip.AddrPort
}

type PeerConnection struct {
	ctx    context.Context
	cancel context.CancelCauseFunc

	// keepAliveSelf *time.Ticker
	keepAlivePeer *time.Timer
	keepAliveFreq time.Duration
	peerTimeout   time.Duration

	conn  net.Conn
	owner *Peer

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
	c.owner = nil
	c.keepAliveFreq = time.Minute
	c.peerTimeout = time.Minute * 3
	c.out = make(chan peerMessage)
	c.in = make(chan peerMessage)
	c.queue = make(chan PeerRequest)

	c.conn = conn

	return &c
}

func (c *PeerConnection) handshakePeer(infohash [20]byte, clientPid PeerID) (PeerID, error) {
	err := utils.SendHandshake(c.conn, infohash, clientPid)
	if err != nil {
		return [20]byte{}, err
	}

	pid, ih, ok := utils.ReceiveHandshake(c.conn)
	if !ok || ih != infohash {
		return [20]byte{}, Peer_bad_handshake_err
	}

	return pid, nil
}

func (c *PeerConnection) handshakeIncomingPeer(torrents map[[20]byte]*Torrent, clientPid PeerID) (PeerID, *Torrent, error) {
	pid, ih, ok := utils.ReceiveHandshake(c.conn)
	if !ok {
		return [20]byte{}, nil, Peer_bad_handshake_err
	}

	torrent, ok := torrents[ih]
	if !ok {
		return [20]byte{}, nil, Peer_bad_handshake_err
	}

	err := utils.SendHandshake(c.conn, ih, clientPid)

	if err != nil {
		return [20]byte{}, nil, err
	}

	return pid, torrent, nil
}

func (c *PeerConnection) attachPeer(p *Peer) {
	c.ctx, c.cancel = context.WithCancelCause(p.ctx)
	c.owner = p
}

func (p *PeerConnection) start() {
	if p.owner == nil {
		panic("peer connection started without logical peer attached")
	}
	// p.keepAliveSelf = time.NewTicker(p.keepAliveFreq)
	p.keepAlivePeer = time.NewTimer(p.peerTimeout)
	go p.loop()
}

func (p *PeerConnection) loop() {
	go p.receiveLoop()

	for {
		select {
		case <-p.ctx.Done():
			{
				if p.owner.Pid == nil {
					fmt.Printf(
						"ANONYMOUS PEER DISCONNECTED BECAUSE: %v\n",
						context.Cause(p.ctx).Error())
				} else {
					fmt.Printf(
						"%v DISCONNECTED BECAUSE: %v\n",
						p.owner.Pid.String(), context.Cause(p.ctx).Error())
				}
				p.owner.CloseConnection()
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

func (p *PeerConnection) keepAlive() (time.Time, bool) {
	ka := peerMessage{
		Peer:    p,
		Kind:    KeepAlive,
		Payload: nil,
	}

	if p == nil || p.ctx.Err() != nil {
		return time.Now(), false
	}

	go p.send(ka)

	return time.Now().Add(time.Minute), true
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
			fmt.Printf("KEEP ALIVE RECEIVED -> %v\n", p.owner.Pid.String())
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
			p.owner.in <- mess
		}
	case Bitfield:
		{
			if p.bitfieldSent {
				p.cancel(Peer_double_bitfield)
				return
			}
			p.bitfieldSent = true
			p.owner.in <- mess
		}
	default:
		{
			p.owner.in <- mess
		}
	}
}
