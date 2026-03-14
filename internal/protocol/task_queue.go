package protocol

import (
	"container/heap"
	"time"
)

type Task struct {
	RunAt time.Time
	fn    func() (time.Time, bool)
}

type Item struct {
	task Task
	idx  int
}

type taskPQueue []*Item

func (pq taskPQueue) Len() int { return len(pq) }

func (pq taskPQueue) Less(i, j int) bool {
	return pq[i].task.RunAt.Before(pq[j].task.RunAt)
}

func (pq taskPQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].idx = i
	pq[j].idx = j
}

func (pq *taskPQueue) Push(x any) {
	n := len(*pq)
	item := x.(*Item)
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

func (pq *taskPQueue) update(item *Item, task Task) {
	item.task = task
	heap.Fix(pq, item.idx)
}
