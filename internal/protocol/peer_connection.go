package protocol

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"time"
)

type PeerConnection struct {
	sock net.Conn

	In  chan PeerMessage
	Out chan PeerMessage

	PeerInfo     Peer
	IsChoked     bool
	IsInterested bool
	AmChoked     bool
	AmInterested bool
}

func NewPeerConn(p Peer, ih [20]byte, pid [20]byte) (PeerConnection, error) {
	handle := PeerConnection{}
	handle.PeerInfo = p
	handle.AmChoked = true
	handle.IsChoked = true
	handle.AmInterested = false
	handle.IsInterested = false
	handle.In = make(chan PeerMessage)
	handle.Out = make(chan PeerMessage)

	conn, err := net.DialTimeout("tcp", p.IpPort.String(), time.Second*5)
	if err != nil {
		return PeerConnection{}, err
	}

	handshakeReq, err := sendHandshake(conn, ih, pid)
	if err != nil {
		return PeerConnection{}, err
	}

	err = receiveHandshake(conn, handshakeReq)
	if err != nil {
		return PeerConnection{}, err
	}

	handle.sock = conn

	go handle.readLoop()
	go handle.writeLoop()

	return handle, nil
}

func (p *PeerConnection) writeLoop() {
	for mess := range p.Out {
		err := p.send(mess)
		if err != nil {
			return
		}
	}
}

func (p *PeerConnection) readLoop() {
	for {
		mess, err := p.receive()
		if err != nil {
			return
		}
		p.In <- mess
	}
}

func (p *PeerConnection) receive() (PeerMessage, error) {
	lenBuf := make([]byte, 4)

	p.sock.SetDeadline(time.Now().Add(time.Second * 5))
	bytesRead, err := io.ReadFull(p.sock, lenBuf)
	if err != nil || bytesRead < 4 {
		return PeerMessage{}, err
	}

	length := binary.BigEndian.Uint32(lenBuf)
	messBuf := make([]byte, length)

	bytesRead, err = io.ReadFull(p.sock, messBuf)
	p.sock.SetDeadline(time.Time{})

	if err != nil || bytesRead < int(length) {
		return PeerMessage{}, err
	}

	input := []byte{}
	input = append(input, lenBuf...)
	input = append(input, messBuf...)

	mess, err := fromNetwork(input)
	return mess, err
}

func (p *PeerConnection) send(mess PeerMessage) error {
	// if mess.Kind < Have && mess.Payload != nil ||
	// 	mess.Kind > Uninterested && mess.Payload == nil {
	// 	return Bad_peer_message_err
	// }
	//
	// output := []byte{}
	// length := make([]byte, 4)
	//
	// binary.BigEndian.PutUint32(length, uint32(len(mess.Payload)+1))
	//
	// output = append(output, length...)
	// output = append(output, byte(mess.Kind))

	// content := []byte{}
	//
	// switch mess.Kind {
	// case Have:
	// 	{
	// 		if len(mess.Payload) != 4 {
	// 			return Bad_peer_message_err
	// 		}
	// 		pidx := binary.LittleEndian.Uint32(mess.Payload)
	//
	// 		content = binary.BigEndian.AppendUint32(content, pidx)
	// 	}
	// case Bitfield:
	// 	{
	// 		content = mess.Payload
	// 	}
	// case Request:
	// 	{
	// 		if len(mess.Payload) != 13 {
	// 			return Bad_peer_message_err
	// 		}
	// 		idx := binary.LittleEndian.Uint32(mess.Payload[:4])
	// 		begin := binary.LittleEndian.Uint32(mess.Payload[4:8])
	// 		length := binary.LittleEndian.Uint32(mess.Payload[8:12])
	//
	// 		content = binary.BigEndian.AppendUint32(content, idx)
	// 		content = binary.BigEndian.AppendUint32(content, begin)
	// 		content = binary.BigEndian.AppendUint32(content, length)
	// 	}
	// case Piece:
	// 	{
	// 		if len(mess.Payload) < 9 {
	// 			return Bad_peer_message_err
	// 		}
	// 		idx := binary.LittleEndian.Uint32(mess.Payload[:4])
	// 		begin := binary.LittleEndian.Uint32(mess.Payload[4:8])
	// 		block := mess.Payload[8:]
	//
	// 		if len(block)+9 != len(mess.Payload) {
	// 			return Bad_peer_message_err
	// 		}
	//
	// 		content = binary.BigEndian.AppendUint32(content, idx)
	// 		content = binary.BigEndian.AppendUint32(content, begin)
	// 		content = append(content, block...)
	// 	}
	// case Cancel:
	// 	{
	// 		if len(mess.Payload) != 13 {
	// 			return Bad_peer_message_err
	// 		}
	// 		idx := binary.LittleEndian.Uint32(mess.Payload[:4])
	// 		begin := binary.LittleEndian.Uint32(mess.Payload[4:8])
	// 		length := binary.LittleEndian.Uint32(mess.Payload[8:12])
	//
	// 		content = binary.BigEndian.AppendUint32(content, idx)
	// 		content = binary.BigEndian.AppendUint32(content, begin)
	// 		content = binary.BigEndian.AppendUint32(content, length)
	// 	}
	// }
	//

	content, err := mess.ToNetwork()
	if err != nil {
		return err
	}
	return WriteFull(p.sock, content)
}

func receiveHandshake(conn net.Conn, req []byte) error {
	buf := make([]byte, 68)

	conn.SetDeadline(time.Now().Add(time.Second * 5))
	bytesRead, err := io.ReadFull(conn, buf)
	conn.SetDeadline(time.Time{})

	if err != nil {
		return Bad_peer_handshake_err
	}
	if bytesRead < 68 {
		return Bad_peer_handshake_err
	}

	if !bytes.Equal(buf[:20], req[:20]) {
		return Bad_peer_handshake_err
	}

	if !bytes.Equal(buf[29:49], req[29:49]) {
		return Bad_peer_handshake_err
	}
	return nil
}

func sendHandshake(conn net.Conn, ih, pid [20]byte) ([]byte, error) {
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

	err := WriteFull(conn, handshake)
	if err != nil {
		return []byte{}, err
	}

	return handshake, nil
}
