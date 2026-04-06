package protocol

type DiskEvent interface {
	IsDiskEvent()
	Event
}

type DiskWriteFinished struct {
	PieceIdx uint32
	BlockOff uint32
	Err      error
}

type DiskReadFinished struct {
	RequestedFrom PeerID
	PieceIdx      uint32
	BlockOff      uint32
	Data          []byte
	Err           error
}

type DiskHashFinished struct {
	PieceIdx uint32
	Err      error
}

func (DiskWriteFinished) IsEvent()     {}
func (DiskReadFinished) IsEvent()      {}
func (DiskHashFinished) IsEvent()      {}
func (DiskWriteFinished) IsDiskEvent() {}
func (DiskReadFinished) IsDiskEvent()  {}
func (DiskHashFinished) IsDiskEvent()  {}
