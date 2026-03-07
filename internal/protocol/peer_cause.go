package protocol

import "errors"

var (
	Peer_timeout             = errors.New("peer timeout")
	Peer_read_err            = errors.New("peer reading error")
	Peer_write_err           = errors.New("peer writing error")
	Peer_disconnected        = errors.New("peer disconnected")
	Peer_malformed_mess_sent = errors.New("malformed message sent")
	Peer_malformed_mess_recv = errors.New("malformed message received")
	Peer_double_bitfield     = errors.New("double bitfield received")
	Peer_invalid_bitfield    = errors.New("invalid bitfield received")
)
