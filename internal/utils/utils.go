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

	// fmt.Printf("bytes before -> %v\nbytes after -> %v\n", len(b.Words())*8, len(out))

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
