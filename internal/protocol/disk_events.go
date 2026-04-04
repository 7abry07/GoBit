package protocol

type DiskEvent interface {
	IsDiskEvent()
	Event
}

type DiskWriteSuccessful struct {
	PieceIdx uint32
	BlockOff uint32
}

type DiskWriteFailed struct {
	PieceIdx uint32
	BlockOff uint32
	Err      error
}

type DiskReadSuccessful struct {
	RequestedFrom PeerID
	PieceIdx      uint32
	BlockOff      uint32
	Data          []byte
}

type DiskReadFailed struct {
	RequestedFrom PeerID
	PieceIdx      uint32
	BlockOff      uint32
	Err           error
}

type DiskHashPassed struct {
	PieceIdx uint32
}

type DiskHashFailed struct {
	PieceIdx uint32
	Err      error
}

func (ev DiskWriteSuccessful) IsEvent()     {}
func (ev DiskWriteFailed) IsEvent()         {}
func (ev DiskReadSuccessful) IsEvent()      {}
func (ev DiskReadFailed) IsEvent()          {}
func (ev DiskHashPassed) IsEvent()          {}
func (ev DiskHashFailed) IsEvent()          {}
func (ev DiskWriteSuccessful) IsDiskEvent() {}
func (ev DiskWriteFailed) IsDiskEvent()     {}
func (ev DiskReadSuccessful) IsDiskEvent()  {}
func (ev DiskReadFailed) IsDiskEvent()      {}
func (ev DiskHashPassed) IsDiskEvent()      {}
func (ev DiskHashFailed) IsDiskEvent()      {}
