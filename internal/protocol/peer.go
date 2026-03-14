package protocol

import (
	"GoBit/internal/utils"
	"context"
	"encoding/binary"
	// "fmt"
	// "math"
	"net"
	"time"
)

type PeerConnectionStatus int

const (
	CONNECTED PeerConnectionStatus = iota
	CLOSED
)

type Peer struct {
	Pid    *PeerID
	ctx    context.Context
	cancel context.CancelCauseFunc

	failureCnt int

	Endpoint PeerEndpoint
	State    PeerConnectionStatus
	Conn     *PeerConnection

	in      chan peerMessage
	torrent *Torrent
	Pieces  *utils.Bitfield

	clientPid PeerID
}

func NewPeer(t *Torrent, e PeerEndpoint, clientPid PeerID) *Peer {
	peer := Peer{}
	peer.ctx, peer.cancel = context.WithCancelCause(t.ctx)

	peer.Endpoint = e
	peer.Conn = nil
	peer.in = make(chan peerMessage)
	peer.Pieces = utils.NewBitfield(uint32(len(t.Info.Pieces) / 20))
	peer.State = CLOSED

	peer.torrent = t
	peer.Pid = e.Pid

	peer.clientPid = clientPid

	go peer.loop()

	return &peer
}

func (p *Peer) TryConnect() error {
	if p.Conn != nil {
		return nil
	}

	conn, err := net.DialTimeout("tcp", p.Endpoint.IpPort.String(), time.Second*3)
	if err != nil {
		p.failureCnt++
		return err
	}
	c := newPeerConnection(conn)
	pid, err := c.handshakePeer(p.torrent.Info.InfoHash, p.clientPid)

	if err != nil {
		p.failureCnt++
		return err
	}
	if (p.Pid != nil) && (pid != *p.Pid) {
		p.failureCnt++
		return Peer_id_mismatch_err
	}

	p.State = CONNECTED
	p.Pid = &pid
	p.Conn = c
	c.attachPeer(p)

	c.start()

	return nil
}

func (p *Peer) loop() {
	for {
		select {
		case <-p.ctx.Done():
			{
				p.torrent.RemovePeer(p)
				return
			}
		case mess := <-p.in:
			go p.handleMessage(mess)
		}
	}
}

func (p *Peer) handleMessage(mess peerMessage) {
	switch mess.Kind {
	case Bitfield:
		{
			ok := p.Pieces.SetBitfield(mess.Payload)
			if !ok {
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.torrent.ReceiveMessage(mess)
		}
	case Have:
		{
			idx := binary.LittleEndian.Uint32(mess.Payload)
			ok := p.Pieces.Set(idx, true)
			if !ok {
				p.cancel(Peer_malformed_mess_recv)
				return
			}
			p.torrent.ReceiveMessage(mess)
		}
	}
}

func (p *Peer) SendBlock(res PeerBlockResponse) {
	if p.Conn == nil {
		return
	}
	mess := peerMessage{}
	mess.Kind = Piece
	binary.LittleEndian.AppendUint32(mess.Payload, res.Idx)
	binary.LittleEndian.AppendUint32(mess.Payload, res.Begin)
	mess.Payload = append(mess.Payload, res.Block...)
	p.Conn.out <- mess
}

func (p *Peer) SendHave(idx uint32) {
	if p.Conn == nil {
		return
	}
	mess := peerMessage{}
	mess.Kind = Have
	binary.LittleEndian.AppendUint32(mess.Payload, idx)
	p.Conn.out <- mess
}

func (p *Peer) SendBitfield(bitfield *utils.Bitfield) {
	if p.Conn == nil {
		return
	}
	mess := peerMessage{}
	mess.Kind = Bitfield
	mess.Payload = bitfield.Data()
	p.Conn.out <- mess
}

func (p *Peer) SendInterested(v bool) {
	if p.Conn == nil {
		return
	}
	mess := peerMessage{}
	if v {
		mess.Kind = Interested
	} else {
		mess.Kind = Interested
	}
	mess.Payload = nil
	p.Conn.out <- mess
}

func (p *Peer) SendChoked(v bool) {
	if p.Conn == nil {
		return
	}
	mess := peerMessage{}
	if v {
		mess.Kind = Choke
	} else {
		mess.Kind = Unchoke
	}
	mess.Payload = nil
	p.Conn.out <- mess
}

func (p *Peer) CancelRequest(req PeerRequest) {
	if p.Conn == nil {
		return
	}
	mess := peerMessage{}
	mess.Kind = Cancel
	binary.LittleEndian.AppendUint32(mess.Payload, req.Idx)
	binary.LittleEndian.AppendUint32(mess.Payload, req.Begin)
	binary.LittleEndian.AppendUint32(mess.Payload, req.Length)
	p.Conn.out <- mess
}

func (p *Peer) CloseConnection() {
	p.State = CLOSED
	p.Conn = nil
}
