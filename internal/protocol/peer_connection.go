package protocol

import (
	"GoBit/internal/utils"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type peerConnection struct {
	ctx    context.Context
	cancel context.CancelFunc

	sock net.Conn
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

func (p *peerConnection) loop() {
	go p.receiveLoop()

	for {
		select {
		case <-p.ctx.Done():
			{
				fmt.Printf("PEER REMOVED -> %v\n", string(p.Info.Pid.String()))
				p.sock.Close()
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
	return utils.WriteFull(p.sock, content)
}

func (p *peerConnection) receiveLoop() {
	for {
		lenBuf := make([]byte, 4)

		bytesRead, err := io.ReadFull(p.sock, lenBuf)
		if err != nil || bytesRead < 4 {
			p.cancel()
			return
		}

		length := binary.BigEndian.Uint32(lenBuf)
		messBuf := make([]byte, length)

		bytesRead, err = io.ReadFull(p.sock, messBuf)

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
