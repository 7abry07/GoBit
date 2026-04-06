package protocol

import "errors"

var (
	Peer_timeout           = errors.New("peer timeout")
	Peer_read_err          = errors.New("peer reading error")
	Peer_write_err         = errors.New("peer writing error")
	Peer_redundant         = errors.New("seed - seed connections are redundant")
	Peer_too_many_attempts = errors.New("too many connection attempts")
)
