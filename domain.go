package asyncqu

import (
	"context"
)

const (
	Start = StepName("start")
	End   = StepName("end")
)

type AsyncJobExecutor interface {
	Append(step StepName, job AsyncJobCallFn, clauses ...StepName)
	AddEnd(steps ...StepName)
	AsyncRun(ctx context.Context) error
	Wait() error
	IsDone() bool
	Errs() []error
}

type AsyncJobCallFn func(ctx context.Context, step StepName) error
