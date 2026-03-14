package protocol

import (
	"errors"
	"fmt"
)

type PeerID [20]byte
type __ PeerID

func NewPeerID(pid []byte) (*PeerID, error) {
	if len(pid) != 20 {
		return nil, errors.New("peer id is not 20 bytes")
	}
	p := (PeerID)(([20]byte)(pid))
	return &p, nil
}

func (pid PeerID) String() string {
	return fmt.Sprintf("%q", __(pid))
}
