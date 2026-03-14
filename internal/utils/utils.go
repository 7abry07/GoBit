package utils

import (
	"bytes"
	"io"
	"math/rand"
	"net"
	"time"
)

func WriteFull(conn net.Conn, content []byte) error {
	writtenCnt := 0
	for writtenCnt < len(content) {
		written, err := conn.Write(content)
		if err != nil {
			return err
		}
		writtenCnt += written
	}
	return nil
}

func SendHandshake(conn net.Conn, ih, pid [20]byte) error {
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
		return err
	}

	return nil
}

func ReceiveHandshake(conn net.Conn) ([20]byte, [20]byte, bool) {
	handshake := []byte{
		0x13,
		'B', 'i', 't',
		'T', 'o', 'r', 'r', 'e', 'n', 't',
		' ',
		'p', 'r', 'o', 't', 'o', 'c', 'o', 'l',
	}

	buf := make([]byte, 68)

	conn.SetDeadline(time.Now().Add(time.Second * 10))
	r, err := io.ReadFull(conn, buf)
	conn.SetDeadline(time.Time{})
	pid := [20]byte{}
	ih := [20]byte{}

	if err != nil {
		return pid, ih, false
	}

	if r < 68 {
		return pid, ih, false
	}

	if !bytes.Equal(buf[:20], handshake[:20]) {
		return pid, ih, false
	}

	pid = ([20]byte)(buf[48:])
	ih = ([20]byte)(buf[28:48])
	return pid, ih, true
}

func GenerateRandomPeerId() [20]byte {
	pid := []byte{}
	pid = append(pid, []byte("-GB0001-")...)

	randomNumbers := make([]byte, 12)
	for i := range randomNumbers {
		randomNumbers[i] = byte(rand.Intn(10) + '0')
	}

	pid = append(pid, randomNumbers...)

	return [20]byte(pid)
}
