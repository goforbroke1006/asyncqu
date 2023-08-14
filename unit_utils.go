package asyncqu

import (
	"sync"
)

func NewStageVisitSpy() *stageVisitSpy {
	return &stageVisitSpy{}
}

type stageVisitSpy struct {
	narrative   []StageName
	narrativeMx sync.RWMutex
}

func (spy *stageVisitSpy) Append(stage StageName) {
	spy.narrativeMx.Lock()
	spy.narrative = append(spy.narrative, stage)
	spy.narrativeMx.Unlock()
}

func (spy *stageVisitSpy) Len() int {
	spy.narrativeMx.RLock()
	length := len(spy.narrative)
	spy.narrativeMx.RUnlock()

	return length
}

func (spy *stageVisitSpy) At(index int) StageName {
	spy.narrativeMx.RLock()
	name := spy.narrative[index]
	spy.narrativeMx.RUnlock()

	return name
}
