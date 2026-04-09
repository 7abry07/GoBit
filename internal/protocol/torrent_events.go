package protocol

type TorrentEvent interface {
	IsTorrentEvent()
	Event
}

type TorrentStarted struct{}
type TorrentFinished struct{}

type PieceCompleted struct {
	Idx uint32
}

type RequestTimeout struct {
	BadPeer PeerID
	Idx     uint32
	Begin   uint32
	Length  uint32
}

type RescheduleBlock struct {
	BadPeer PeerID
	Idx     uint32
	Begin   uint32
	Length  uint32
}

func (TorrentStarted) IsEvent()  {}
func (TorrentFinished) IsEvent() {}
func (PieceCompleted) IsEvent()  {}
func (RequestTimeout) IsEvent()  {}
func (RescheduleBlock) IsEvent() {}

func (TorrentStarted) IsTorrentEvent()  {}
func (TorrentFinished) IsTorrentEvent() {}
func (PieceCompleted) IsTorrentEvent()  {}
func (RequestTimeout) IsTorrentEvent()  {}
func (RescheduleBlock) IsTorrentEvent() {}
