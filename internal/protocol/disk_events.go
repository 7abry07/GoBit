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

// type DiskWriteSuccessful struct {
// 	PieceIdx uint32
// 	BlockOff uint32
// }
//
// type DiskWriteFailed struct {
// 	PieceIdx uint32
// 	BlockOff uint32
// 	Err      error
// }
//
// type DiskReadSuccessful struct {
// 	RequestedFrom PeerID
// 	PieceIdx      uint32
// 	BlockOff      uint32
// 	Data          []byte
// }
//
// type DiskReadFailed struct {
// 	RequestedFrom PeerID
// 	PieceIdx      uint32
// 	BlockOff      uint32
// 	Err           error
// }
//
// type DiskHashPassed struct {
// 	PieceIdx uint32
// }
//
// type DiskHashFailed struct {
// 	PieceIdx uint32
// 	Err      error
// }

// func (DiskWriteSuccessful) IsEvent()     {}
// func (DiskWriteFailed) IsEvent()         {}
// func (DiskReadSuccessful) IsEvent()      {}
// func (DiskReadFailed) IsEvent()          {}
// func (DiskHashPassed) IsEvent()          {}
// func (DiskHashFailed) IsEvent()          {}
// func (DiskWriteSuccessful) IsDiskEvent() {}
// func (DiskWriteFailed) IsDiskEvent()     {}
// func (DiskReadSuccessful) IsDiskEvent()  {}
// func (DiskReadFailed) IsDiskEvent()      {}
// func (DiskHashPassed) IsDiskEvent()      {}
// func (DiskHashFailed) IsDiskEvent()      {}
