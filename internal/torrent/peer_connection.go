package torrent

import (
	"bytes"
	"io"
	"net"
	"time"
)

type PeerConnection struct {
	sock net.Conn

	PeerInfo     Peer
	IsChoked     bool
	IsInterested bool
	AmChoked     bool
	AmInterested bool
}

func NewPeerConn(p Peer, ih [20]byte, pid [20]byte) (PeerConnection, error) {
	handle := PeerConnection{}
	handle.AmChoked = true
	handle.IsChoked = true
	handle.AmInterested = false
	handle.IsInterested = false
	handle.PeerInfo = p

	conn, err := net.DialTimeout("tcp", p.IpPort.String(), time.Second*5)
	if err != nil {
		return PeerConnection{}, err
	}

	handshakeReq := []byte{
		0x13,
		'B', 'i', 't',
		'T', 'o', 'r', 'r', 'e', 'n', 't',
		' ',
		'p', 'r', 'o', 't', 'o', 'c', 'o', 'l',
		0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0,
	}
	handshakeReq = append(handshakeReq, ih[:]...)
	handshakeReq = append(handshakeReq, pid[:]...)

	if len(handshakeReq) != 68 {
		panic("bad handshake length")
	}

	_, err = conn.Write(handshakeReq)
	if err != nil {
		panic(err)
	}

	buf := make([]byte, 68)

	conn.SetDeadline(time.Now().Add(time.Second * 5))
	bytesRead, err := io.ReadFull(conn, buf)
	conn.SetDeadline(time.Time{})

	if err != nil {
		return PeerConnection{}, err
	}
	if bytesRead < 68 {
		return PeerConnection{}, Bad_peer_handshake_err
	}

	if !bytes.Equal(buf[:20], handshakeReq[:20]) {
		return PeerConnection{}, Bad_peer_handshake_err
	}

	if !bytes.Equal(buf[29:49], handshakeReq[29:49]) {
		return PeerConnection{}, Bad_peer_handshake_err
	}
	handle.sock = conn
	return handle, nil
}
