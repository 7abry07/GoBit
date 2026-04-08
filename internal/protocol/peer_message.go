package protocol

import (
	"encoding/binary"
)

type peerMessageType uint8

const (
	Choke peerMessageType = iota
	Unchoke
	Interested
	Uninterested
	Have
	Bitfield
	Request
	Piece
	Cancel
	KeepAlive
)

type peerMessage struct {
	Peer    *PeerConnection
	Kind    peerMessageType
	Payload []byte
}

func fromNetwork(input []byte) (peerMessage, error) {
	mess := peerMessage{}
	if len(input) < 4 {
		return peerMessage{}, Peer_bad_message_err
	}

	// length :=
	// lengthBytes := make([]byte, 4)
	// binary.BigEndian.PutUint32(lengthLE, binary.BigEndian.Uint32(input[0:4]))

	length := binary.BigEndian.Uint32(input[0:4])

	if len(input) < int(length)+4 {
		return peerMessage{}, Peer_bad_message_err
	}

	if length == 0 {
		mess.Kind = KeepAlive
		mess.Payload = nil
		return mess, nil
	}

	kind := peerMessageType(input[4])
	mess.Kind = kind
	if kind < Have {
		mess.Payload = nil
	}

	switch kind {
	case Have:
		{
			if length != 5 || len(input) != 9 {
				return peerMessage{}, Peer_bad_message_err
			}
			mess.Kind = Have
			pidx := binary.BigEndian.Uint32(input[5:])
			mess.Payload = binary.BigEndian.AppendUint32(mess.Payload, pidx)
		}
	case Bitfield:
		{
			mess.Kind = Bitfield
			mess.Payload = input[5:]
		}
	case Request:
		{
			if length != 13 || len(input) != 17 {
				return peerMessage{}, Peer_bad_message_err
			}
			mess.Kind = Request
			idx := binary.BigEndian.Uint32(input[5:9])
			begin := binary.BigEndian.Uint32(input[9:13])
			length_ := binary.BigEndian.Uint32(input[13:17])

			mess.Payload = binary.BigEndian.AppendUint32(mess.Payload, idx)
			mess.Payload = binary.BigEndian.AppendUint32(mess.Payload, begin)
			mess.Payload = binary.BigEndian.AppendUint32(mess.Payload, length_)
		}
	case Piece:
		{
			if length < 9 || len(input) < 13 {
				return peerMessage{}, Peer_bad_message_err
			}
			mess.Kind = Piece
			idx := binary.BigEndian.Uint32(input[5:9])
			begin := binary.BigEndian.Uint32(input[9:13])
			block := input[13:]

			if length != uint32(len(block)+9) || len(input) < len(block)+13 {
				return peerMessage{}, Peer_bad_message_err
			}

			mess.Payload = binary.BigEndian.AppendUint32(mess.Payload, idx)
			mess.Payload = binary.BigEndian.AppendUint32(mess.Payload, begin)
			mess.Payload = append(mess.Payload, block...)
		}
	case Cancel:
		{
			if length != 13 || len(input) != 17 {
				return peerMessage{}, Peer_bad_message_err
			}
			mess.Kind = Cancel
			idx := binary.BigEndian.Uint32(input[5:9])
			begin := binary.BigEndian.Uint32(input[9:13])
			length_ := binary.BigEndian.Uint32(input[13:17])

			mess.Payload = binary.BigEndian.AppendUint32(mess.Payload, idx)
			mess.Payload = binary.BigEndian.AppendUint32(mess.Payload, begin)
			mess.Payload = binary.BigEndian.AppendUint32(mess.Payload, length_)
		}
	}

	return mess, nil
}

func (m peerMessage) ToNetwork() ([]byte, error) {
	output := []byte{}
	length := make([]byte, 4)

	if m.Kind == KeepAlive {
		binary.BigEndian.PutUint32(length, uint32(len(m.Payload)))
		output = append(output, length...)
	} else {
		binary.BigEndian.PutUint32(length, uint32(len(m.Payload)+1))
		output = append(output, length...)
		output = append(output, byte(m.Kind))
	}

	content := []byte{}

	switch m.Kind {
	case Choke:
		fallthrough
	case Unchoke:
		fallthrough
	case Interested:
		fallthrough
	case Uninterested:
		if m.Payload != nil {
			return []byte{}, Peer_bad_message_err
		}
	case Have:
		{
			if len(m.Payload) != 4 {
				return []byte{}, Peer_bad_message_err
			}
			pidx := binary.BigEndian.Uint32(m.Payload)

			content = binary.BigEndian.AppendUint32(content, pidx)
		}
	case Bitfield:
		{
			content = m.Payload
		}
	case Request:
		{
			if len(m.Payload) != 12 {
				return []byte{}, Peer_bad_message_err
			}
			idx := binary.BigEndian.Uint32(m.Payload[:4])
			begin := binary.BigEndian.Uint32(m.Payload[4:8])
			length := binary.BigEndian.Uint32(m.Payload[8:12])

			content = binary.BigEndian.AppendUint32(content, idx)
			content = binary.BigEndian.AppendUint32(content, begin)
			content = binary.BigEndian.AppendUint32(content, length)
		}
	case Piece:
		{
			if len(m.Payload) < 8 {
				return []byte{}, Peer_bad_message_err
			}
			idx := binary.BigEndian.Uint32(m.Payload[:4])
			begin := binary.BigEndian.Uint32(m.Payload[4:8])
			block := m.Payload[8:]

			if len(block)+8 != len(m.Payload) {
				return []byte{}, Peer_bad_message_err
			}

			content = binary.BigEndian.AppendUint32(content, idx)
			content = binary.BigEndian.AppendUint32(content, begin)
			content = append(content, block...)
		}
	case Cancel:
		{
			if len(m.Payload) != 12 {
				return []byte{}, Peer_bad_message_err
			}
			idx := binary.BigEndian.Uint32(m.Payload[:4])
			begin := binary.BigEndian.Uint32(m.Payload[4:8])
			length := binary.BigEndian.Uint32(m.Payload[8:12])

			content = binary.BigEndian.AppendUint32(content, idx)
			content = binary.BigEndian.AppendUint32(content, begin)
			content = binary.BigEndian.AppendUint32(content, length)
		}
	}

	output = append(output, content...)

	return output, nil
}
