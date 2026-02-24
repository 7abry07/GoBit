package protocol

import (
	"errors"
	"fmt"
)

type PeerID [20]byte
type __ PeerID

func NewPeerID(pid []byte) (PeerID, error) {
	if len(pid) != 20 {
		return PeerID{}, errors.New("peer id is not 20 bytes")
	}
	return ([20]byte)(pid), nil
}

func (pid PeerID) String() string {
	return fmt.Sprintf("%q", __(pid))
}
