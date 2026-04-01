package protocol

type ChokerTask interface {
	IsChokerTask()
	Task
}

type ChokerTick struct{}
type OptimisticUnchokeTick struct{}

func (tsk ChokerTick) IsTask()                  {}
func (tsk OptimisticUnchokeTick) IsTask()       {}
func (tsk ChokerTick) IsChokerTask()            {}
func (tsk OptimisticUnchokeTick) IsChokerTask() {}
