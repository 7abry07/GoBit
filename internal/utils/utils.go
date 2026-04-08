package utils

import (
	"math/rand"
	"net"

	"github.com/bits-and-blooms/bitset"
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

func WriteFullUDP(conn *net.UDPConn, addr *net.UDPAddr, data []byte) error {
	total := 0
	for total < len(data) {
		n, err := conn.Write(data[total:])
		if err != nil {
			return err
		}
		total += n
	}
	return nil
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

func BoolToInt(val bool) int {
	if val {
		return 1
	}
	return 0
}

func BitSetToBytes(b bitset.BitSet) []byte {
	byteLen := (b.Len() + 7) / 8
	out := make([]byte, byteLen)

	for i := range b.Len() {
		if b.Test(i) {
			byteIndex := i / 8
			bitIndex := 7 - (i % 8)
			out[byteIndex] |= 1 << bitIndex
		}
	}
	return out
}

func BytesToBitSet(data []byte, bits uint) *bitset.BitSet {
	b := bitset.New(bits)

	for i := range bits {
		byteIndex := i / 8
		bitIndex := 7 - (i % 8)

		if data[byteIndex]&(1<<bitIndex) != 0 {
			b.Set(i)
		}
	}
	return b
}
