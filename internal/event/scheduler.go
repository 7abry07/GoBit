package event

import (
	"container/heap"
	"time"
)

type Scheduler struct {
	taskTimer *time.Timer
	tasks     taskPQueue
	taskChan  chan Task
}

func NewScheduler() *Scheduler {
	s := Scheduler{}
	s.taskTimer = time.NewTimer(0)
	s.taskChan = make(chan Task)
	<-s.taskTimer.C
	heap.Init(&s.tasks)

	go s.loop()
	return &s
}

func (s *Scheduler) Schedule(t Task) {
	s.taskChan <- t
}

func (s *Scheduler) runAndReschedule(task Task) {
	nextTime, repeat := task.Fn()
	if repeat {
		newTask := Task{
			Fn:    task.Fn,
			RunAt: nextTime,
		}
		s.Schedule(newTask)
	}
}

func (s *Scheduler) loop() {
	for {
		select {
		case t := <-s.taskChan:
			{
				item := Item{
					task: t,
				}

				if len(s.tasks) == 0 {
					heap.Push(&s.tasks, &item)
					s.taskTimer.Reset(time.Until(t.RunAt))
					continue
				}

				original := s.tasks[0]
				heap.Push(&s.tasks, &item)

				if original != s.tasks[0] {
					s.taskTimer.Reset(time.Until(item.task.RunAt))
				}
			}
		case <-s.taskTimer.C:
			{
				task := heap.Pop(&s.tasks).(*Item).task

				go s.runAndReschedule(task)

				if len(s.tasks) != 0 {
					newTask := s.tasks[0].task
					s.taskTimer.Reset(time.Until(newTask.RunAt))
				}
			}
		}
	}
}
