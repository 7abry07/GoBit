package protocol

type Torrent struct {
	Info           TorrentFile
	TrackerManager TrackerManager
	PeerList       []*PeerConnection
}
