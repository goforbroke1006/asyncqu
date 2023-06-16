package asyncqu

import (
	"context"
)

type AsyncJobExecutor interface {
	SetOnChanges(cb OnChangedCb)
	Append(step StepName, job AsyncJobCallFn, clauses ...StepName)
	AddEnd(steps ...StepName)
	AsyncRun(ctx context.Context) error
	Wait() error
	IsDone() bool
	Errs() []error
}

type OnChangedCb func(name StepName, state JobState, err error)

type AsyncJobCallFn func(ctx context.Context) error
