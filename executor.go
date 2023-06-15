package asyncqu

import (
	"context"
	"time"
)

func New() AsyncJobExecutor {
	return &executorImpl{
		registered:      []*JobItem{},
		causesDone:      map[StepName]struct{}{},
		allJobsDoneChan: make(chan struct{}),
	}
}

type executorImpl struct {
	registered      []*JobItem
	startedFlag     bool
	causesDone      map[StepName]struct{}
	allJobsDoneChan chan struct{}
	doneFlag        bool
}

func (e *executorImpl) Append(step StepName, job AsyncJobCallFn, clauses ...StepName) {
	e.registered = append(e.registered, &JobItem{
		StepName: step,
		Fn:       job,
		State:    Runnable,
		Causes:   clauses,
	})
}

func (e *executorImpl) AddEnd(steps ...StepName) {
	e.registered = append(e.registered, &JobItem{
		StepName: End,
		Fn:       func(ctx context.Context) error { return nil },
		State:    Runnable,
		Causes:   steps,
	})
}

func (e *executorImpl) hasEnd() bool {
	for _, reg := range e.registered {
		if reg.StepName == End {
			return true
		}
	}
	return false
}

func (e *executorImpl) AsyncRun(ctx context.Context) error {
	if !e.hasEnd() {
		return ErrEndStepIsNotSpecified
	}

	e.startedFlag = true
	e.causesDone[Start] = struct{}{}

	doneCh := make(chan StepName)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case label := <-doneCh:
				e.causesDone[label] = struct{}{}
			}
		}
	}()

	go func() {
		for {
			allDone := true
			for _, item := range e.registered {
				allDone = allDone && item.State == Done

				if item.State == Runnable && e.isCausesDone(item.Causes...) {
					item.State = Running

					go func(ctx context.Context, item *JobItem) {
						execFnCtx := context.WithValue(ctx, "step", item.StepName) //nolint:staticcheck

						if resErr := item.Fn(execFnCtx); resErr != nil {
							item.Err = resErr
						}
						item.State = Done
						doneCh <- item.StepName
					}(ctx, item)
				}
			}

			if allDone {
				break
			}

			time.Sleep(time.Second)
		}

		close(doneCh)
		e.doneFlag = true
		e.allJobsDoneChan <- struct{}{}
	}()

	return nil
}

func (e *executorImpl) Wait() error {
	if !e.startedFlag {
		return ErrExecutorWasNotStarted
	}
	<-e.allJobsDoneChan
	return nil
}

func (e *executorImpl) Errs() []error {
	errs := make([]error, 0)
	for _, item := range e.registered {
		if item.Err != nil {
			errs = append(errs, item.Err)
		}
	}

	return errs
}

func (e *executorImpl) IsDone() bool {
	return e.doneFlag
}

func (e *executorImpl) isCausesDone(steps ...StepName) bool {
	for _, s := range steps {
		if _, exists := e.causesDone[s]; !exists {
			return false
		}
	}
	return true
}
