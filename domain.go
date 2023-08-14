package asyncqu

import "context"

type Executor interface {
	SetOnChanges(cb OnChangedCb)
	Append(stageName StageName, fn StageFn, clauses ...StageName)
	SetFinal(fn StageFn)
	SetEnd(stageNames ...StageName)
	Run(ctx context.Context) error
	Errs() []error
}

type OnChangedCb func(stageName StageName, state State, err error)

type StageFn func(ctx context.Context) error

const ContextKeyStageName = StageName("stage-name")
