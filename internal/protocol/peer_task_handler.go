package protocol

import (
	"net"
	"time"
)

func (t *Torrent) handlePeerTask(tsk PeerTask) {
	switch tsk := tsk.(type) {
	case PeerKeepAlive:
		t.handlePeerKeepAlive(tsk)
	case PeerTryConnection:
		t.handlePeerTryConnection(tsk)
	case PeerCalculateStats:
		t.handlePeerCalculateStats(tsk)
	case RefillRequests:
		t.handleRefillRequests()
	}
}

func (t *Torrent) handlePeerKeepAlive(tsk PeerKeepAlive) {
	peer, ok := t.ActivePeers[tsk.Receiver]
	if !ok {
		return
	}
	peer.KeepAlive()
	// fmt.Printf("KEEP ALIVE SENT -> %v\n", tsk.Receiver)
	t.Sched.Schedule(
		PeerKeepAlive{tsk.Receiver},
		time.Now().Add(time.Second*10))
}

func (t *Torrent) handlePeerTryConnection(tsk PeerTryConnection) {
	go func() {
		conn, err := net.DialTimeout("tcp", tsk.Peer.Endpoint.String(), time.Second*3)
		peerConn := newPeerConnection(conn)
		peerConn.Peer = tsk.Peer

		if err == nil {
			err = peerConn.handshakePeer(t.Info.InfoHash, t.Ses.PeerID)
		}

		if err != nil {
			t.SignalEvent(PeerConnectionFailed{tsk.Peer, err})
		} else {
			t.SignalEvent(PeerConnected{peerConn, tsk.Peer.FailureCnt + 1})
		}
	}()
}

func (t *Torrent) handlePeerCalculateStats(tsk PeerCalculateStats) {
	peer, ok := t.ActivePeers[tsk.Receiver]
	if !ok {
		return
	}
	now := time.Now()
	dt := now.Sub(peer.State.LastTickTime).Seconds()
	deltaDownload := peer.State.TotalDownloaded - peer.State.LastTickDownload
	deltaUpload := peer.State.TotalUploaded - peer.State.LastTickUpload
	instantDrate := float64(deltaDownload) / dt
	instantUrate := float64(deltaUpload) / dt

	const alpha = 0.3
	peer.State.DownloadRate = alpha*instantDrate + (1-alpha)*peer.State.DownloadRate
	peer.State.UploadRate = alpha*instantUrate + (1-alpha)*peer.State.UploadRate

	peer.State.LastTickDownload = peer.State.TotalDownloaded
	peer.State.LastTickUpload = peer.State.TotalUploaded
	peer.State.LastTickTime = now

	t.Sched.Schedule(
		PeerCalculateStats{tsk.Receiver},
		time.Now().Add(time.Second))
}

func (t *Torrent) handleRefillRequests() {
	for _, peer := range t.ActivePeers {
		if !peer.State.AmChoked && peer.State.IsInteresting {
			peer.FillOutstandingRequest()
		}
	}
}
