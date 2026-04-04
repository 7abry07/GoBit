package protocol

type DiskJob interface {
	IsDiskJob()
}

type DiskWriteJob struct {
	PieceIdx uint32
	BlockOff uint32
	Data     []byte
}

type DiskReadJob struct {
	RequestedFrom PeerID
	PieceIdx      uint32
	BlockOff      uint32
	Length        uint32
}

type DiskHashJob struct {
	PieceIdx uint32
}

func (DiskWriteJob) IsDiskJob() {}
func (DiskReadJob) IsDiskJob()  {}
func (DiskHashJob) IsDiskJob()  {}
