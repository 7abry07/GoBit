package protocol

import (
	"net"
	"time"
)

func (t *Torrent) handlePeerTask(tsk PeerTask) {
	switch tsk := tsk.(type) {
	case PeerKeepAliveTsk:
		t.handlePeerKeepAlive(tsk)
	case PeerTryConnectionTsk:
		t.handlePeerTryConnection(tsk)
	case PeerCalculateStatsTsk:
		t.handlePeerCalculateStats(tsk)
	}
}

func (t *Torrent) handlePeerKeepAlive(tsk PeerKeepAliveTsk) {
	peer, ok := t.ActivePeers[tsk.Receiver]
	if !ok {
		return
	}
	peer.Conn.KeepAlive()
	// fmt.Printf("KEEP ALIVE SENT -> %v\n", tsk.Receiver)
	t.Sched.Schedule(
		PeerKeepAliveTsk{tsk.Receiver},
		time.Now().Add(time.Second*10))
}

func (t *Torrent) handlePeerTryConnection(tsk PeerTryConnectionTsk) {
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

func (t *Torrent) handlePeerCalculateStats(tsk PeerCalculateStatsTsk) {
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
		PeerCalculateStatsTsk{tsk.Receiver},
		time.Now().Add(time.Second))
}
