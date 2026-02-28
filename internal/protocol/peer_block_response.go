package protocol

type PeerBlockResponse struct {
	Idx   uint32
	Begin uint32
	Block []byte
}
