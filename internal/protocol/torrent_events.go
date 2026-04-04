package protocol

type TorrentEvent interface {
	IsTorrentEvent()
	Event
}

type TorrentStarted struct{}
type TorrentFinished struct{}

func (TorrentStarted) IsEvent()         {}
func (TorrentFinished) IsEvent()        {}
func (TorrentStarted) IsTorrentEvent()  {}
func (TorrentFinished) IsTorrentEvent() {}
