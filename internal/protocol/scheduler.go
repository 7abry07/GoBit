package protocol

import (
	"container/heap"
	// "fmt"
	"context"
	"time"
)

type scheduledTask struct {
	signalAt time.Time
	t        Task
	idx      int
}

type taskPQueue []*scheduledTask

func (pq taskPQueue) Len() int { return len(pq) }

func (pq taskPQueue) Less(i, j int) bool {
	return pq[i].signalAt.Before(pq[j].signalAt)
}

func (pq taskPQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].idx = i
	pq[j].idx = j
}

func (pq *taskPQueue) Push(x any) {
	n := len(*pq)
	item := x.(*scheduledTask)
	item.idx = n
	*pq = append(*pq, item)
}

func (pq *taskPQueue) Pop() any {
	old := *pq
	n := len(old)

	item := old[n-1]
	old[n-1] = nil
	item.idx = -1
	*pq = old[0 : n-1]

	return item
}

// ----------------------------

type Scheduler struct {
	ctx    context.Context
	cancel context.CancelFunc

	taskTimer *time.Timer
	tasks     taskPQueue
	taskChan  chan scheduledTask
	torrent   *Torrent
	ready     []Task
}

func NewScheduler(torrent *Torrent) *Scheduler {
	s := Scheduler{}
	s.ctx, s.cancel = context.WithCancel(torrent.ctx)

	s.taskTimer = time.NewTimer(0)
	s.taskChan = make(chan scheduledTask, 100)
	s.torrent = torrent
	<-s.taskTimer.C
	heap.Init(&s.tasks)

	go s.loop()
	return &s
}

func (s *Scheduler) Schedule(t Task, signalAt time.Time) {
	if s.ctx.Err() != nil {
		return
	}
	s.taskChan <- scheduledTask{signalAt, t, 0}
}

func (s *Scheduler) loop() {
	for {
		select {
		case <-s.ctx.Done():
			s.taskTimer.Stop()
			return
		default:
		}

		if len(s.ready) > 0 {
			s.torrent.SignalTask(s.ready[0])
			s.ready = s.ready[1:]
		}

		select {
		case newTask := <-s.taskChan:
			{
				if len(s.tasks) == 0 {
					heap.Push(&s.tasks, &newTask)
					s.taskTimer.Reset(time.Until(newTask.signalAt))
					continue
				}

				original := s.tasks[0]
				heap.Push(&s.tasks, &newTask)
				if original != s.tasks[0] {
					if s.taskTimer.Stop() {
						s.taskTimer.Reset(time.Until(newTask.signalAt))
					}
				}
			}
		case <-s.taskTimer.C:
			{
				task := heap.Pop(&s.tasks).(*scheduledTask).t
				s.ready = append(s.ready, task)
				if len(s.tasks) != 0 {
					nextTask := s.tasks[0]
					s.taskTimer.Reset(time.Until(nextTask.signalAt))
				}
			}
		}
	}
}
