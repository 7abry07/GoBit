package protocol

type ChokerTask interface {
	IsChokerTask()
	Task
}

type ChokerTick struct{}
type OptimisticUnchokeTick struct{}

func (ChokerTick) IsTask()                  {}
func (OptimisticUnchokeTick) IsTask()       {}
func (ChokerTick) IsChokerTask()            {}
func (OptimisticUnchokeTick) IsChokerTask() {}
