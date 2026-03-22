package protocol

import (
	"container/heap"
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
	taskTimer *time.Timer
	tasks     taskPQueue
	taskChan  chan scheduledTask
	signalTo  chan Task
	ready     []Task
}

func NewScheduler(signalTo chan Task) *Scheduler {
	s := Scheduler{}
	s.taskTimer = time.NewTimer(0)
	s.taskChan = make(chan scheduledTask, 100)
	s.signalTo = signalTo
	<-s.taskTimer.C
	heap.Init(&s.tasks)

	go s.loop()
	return &s
}

func (s *Scheduler) Schedule(t Task, signalAt time.Time) {
	s.taskChan <- scheduledTask{signalAt, t, 0}
}

func (s *Scheduler) loop() {
	for {
		if len(s.ready) > 0 {
			select {
			case s.signalTo <- s.ready[0]:
				s.ready = s.ready[1:]
			default:
			}
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
					s.taskTimer.Reset(time.Until(newTask.signalAt))
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

// func (s *Scheduler) emitter() {
//
// }
