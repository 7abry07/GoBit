package utils

import (
	"bytes"
	// "fmt"
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

func SendHandshake(conn net.Conn, ih, pid [20]byte) ([]byte, error) {
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

func ReceiveHandshake(conn net.Conn, req []byte) ([20]byte, bool) {
	buf := make([]byte, 68)

	conn.SetDeadline(time.Now().Add(time.Second * 5))
	r, err := io.ReadFull(conn, buf)
	conn.SetDeadline(time.Time{})
	pid := [20]byte{}

	if err != nil {
		return ([20]byte)(pid), false
	}

	if r < 68 {
		return ([20]byte)(pid), false
	}

	if !bytes.Equal(buf[:20], req[:20]) {
		return ([20]byte)(pid), false
	}

	if !bytes.Equal(buf[28:48], req[28:48]) {
		return ([20]byte)(pid), false
	}

	pid = ([20]byte)(buf[48:])
	// fmt.Printf("PEER ID -> %v\n", string(pid[:]))
	return pid, true
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
