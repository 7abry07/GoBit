package protocol

import (
	"math/rand/v2"
	"time"
)

func (t *Torrent) handleChokerTask(tsk ChokerTask) {
	switch tsk := tsk.(type) {
	case ChokerTick:
		t.handleChokerTick(tsk)
	case OptimisticUnchokeTick:
		t.handleOptimisticUnchokeTick(tsk)
	}
}

func (t *Torrent) handleChokerTick(tsk ChokerTick) {
	activePeers := []ActivePeer{}
	for _, val := range t.ActivePeers {
		activePeers = append(activePeers, val)
	}

	peersToUnchoke := UnchokeSort(activePeers)
	for i, peer := range activePeers {
		if i < peersToUnchoke {
			t.Unchoke(peer)
			if peer.State.IsOptimistic {
				optimistic := t.ActivePeers[*t.optimisticUnchoke]
				optimistic.State.IsOptimistic = false
				t.optimisticUnchoke = nil
				// fmt.Printf("UNCHOKED -> %v (previously optimistic)\n", peer.Conn.Pid)
			} else {
				// fmt.Printf("UNCHOKED -> %v (download: %.1fkb | upload: %.1fkb)\n", peer.Conn.Pid, peer.State.DownloadRate/1024, peer.State.UploadRate/1024)
			}
		} else {
			if !peer.State.IsOptimistic {
				t.Choke(peer)
			}
		}
	}

	t.Sched.Schedule(ChokerTick{}, time.Now().Add(time.Second*10))
}

func (t *Torrent) handleOptimisticUnchokeTick(tsk OptimisticUnchokeTick) {
	chokedPeers := []ActivePeer{}

	for _, peer := range t.ActivePeers {
		if peer.State.IsChoked {
			chokedPeers = append(chokedPeers, peer)
		}
	}

	if len(chokedPeers) != 0 {
		optimistic := chokedPeers[rand.IntN(len(chokedPeers))]
		if t.optimisticUnchoke != nil {
			if t.optimisticUnchoke != nil {
				previousOptimistic := t.ActivePeers[*t.optimisticUnchoke]
				previousOptimistic.State.IsOptimistic = false
			}
		}
		t.optimisticUnchoke = &optimistic.Conn.Pid
		optimistic.State.IsOptimistic = true
		t.Unchoke(optimistic)
		// fmt.Printf("OPTIMISTICALLY UNCHOKED -> %v\n", optimistic.Conn.Pid)
	}

	t.Sched.Schedule(
		OptimisticUnchokeTick{},
		time.Now().Add(time.Second*30),
	)
}
