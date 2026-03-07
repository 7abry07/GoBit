package utils

import "math"

type Bitfield struct {
	length  uint32
	count   uint32
	payload []uint8
}

func NewBitfield(size uint32) *Bitfield {
	length := uint32(math.Ceil(float64(size) / 8.0))
	count := size
	payload := make([]uint8, length)
	b := Bitfield{length: length, payload: payload, count: count}
	return &b
}

func (b *Bitfield) SetBitfield(payload []uint8) bool {
	if len(payload) != int(b.length) {
		return false
	}
	b.payload = payload
	return true
}

func (b *Bitfield) Set(idx uint32, val bool) bool {
	byteIdx := idx / 8
	bitIdx := 7 - (idx % 8)
	if byteIdx > b.length-1 {
		return false
	}
	if val {
		b.payload[byteIdx] |= 1 >> bitIdx
	} else {
		b.payload[byteIdx] &^= 1 >> bitIdx
	}
	return true
}

func (b *Bitfield) IsSet(idx uint32) bool {
	byteIdx := idx / 8
	bitIdx := 7 - (idx % 8)
	return ((b.payload[byteIdx] >> bitIdx) & 1) == 1
}

func (b *Bitfield) Data() []uint8 {
	return b.payload
}
func (b *Bitfield) Length() uint32 {
	return b.length
}
func (b *Bitfield) Count() uint32 {
	return b.count
}
