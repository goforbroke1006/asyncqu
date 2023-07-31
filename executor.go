package asyncqu

import (
	"context"
	"fmt"
	"sync"
)

func New() Executor {
	return &executorImpl{
		registered:      map[StageName]*StageMeta{},
		startedFlag:     false,
		causesDone:      map[StageName]struct{}{},
		allJobsDoneChan: make(chan struct{}),
		doneFlag:        false,
		onChangesCb:     func(name StageName, state State, err error) {},
		finalCb:         nil,
	}
}

type executorImpl struct {
	sync.RWMutex

	registered      map[StageName]*StageMeta
	startedFlag     bool
	causesDone      map[StageName]struct{}
	allJobsDoneChan chan struct{}
	doneFlag        bool
	onChangesCb     OnChangedCb
	finalCb         StageFn
}

func (e *executorImpl) SetOnChanges(cb OnChangedCb) {
	e.Lock()
	defer e.Unlock()

	e.onChangesCb = cb
}

func (e *executorImpl) Append(stageName StageName, fn StageFn, causes ...StageName) {
	e.Lock()
	defer e.Unlock()

	if _, exists := e.registered[stageName]; exists {
		panic(fmt.Errorf("stage with name '%s' already exists", stageName))
	}

	for _, c := range causes {
		if c == stageName {
			panic(ErrStageShouldNotWaitForItself)
		}
		if c != Start {
			if _, exists := e.registered[c]; !exists {
				panic(ErrStageWaitForUnknown)
			}
		}
	}

	item := &StageMeta{
		Name:           stageName,
		Fn:             fn,
		State:          Runnable,
		Causes:         causes,
		CausesRequired: true,
	}
	e.registered[stageName] = item
	e.onChangesCb(item.Name, item.State, nil)
}

func (e *executorImpl) SetEnd(causes ...StageName) {
	e.Lock()
	defer e.Unlock()

	e.registered[End] = &StageMeta{
		Name:           End,
		Fn:             func(ctx context.Context) error { return nil },
		State:          Runnable,
		Causes:         causes,
		CausesRequired: true,
	}
}

func (e *executorImpl) SetFinal(job StageFn) {
	e.Lock()
	defer e.Unlock()

	e.finalCb = job
}

func (e *executorImpl) hasEnd() bool {
	e.RLock()
	defer e.RUnlock()

	for _, reg := range e.registered {
		if reg.Name == End {
			return true
		}
	}
	return false
}

func (e *executorImpl) AsyncRun(ctx context.Context) error {
	if !e.hasEnd() {
		return ErrEndStageIsNotSpecified
	}

	e.Lock()
	e.startedFlag = true
	e.causesDone[Start] = struct{}{}
	e.Unlock()

	var (
		allSkippedCh = make(chan struct{})
		doneCh       = make(chan StageName)
		execNextCh   = make(chan struct{})
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, isOpen := <-allSkippedCh:
				if !isOpen {
					break
				}
				execNextCh <- struct{}{}
			case stageName, isOpen := <-doneCh:
				if !isOpen {
					break
				}

				e.Lock()
				e.causesDone[stageName] = struct{}{}
				e.Unlock()

				execNextCh <- struct{}{}
			}
		}
	}()

	go func() {
	ExecLoop:
		for {
			select {
			case <-ctx.Done():
				return
			case <-execNextCh:
				allDone := true
				skippedCount := 0

				for _, item := range e.registered {
					allDone = allDone && (item.State == Done || item.State == Skipped)

					if item.State == Runnable && item.CausesRequired && e.isAnyCausesFailedOrSkipped(item.Causes...) {
						item.State = Skipped
						e.onChangesCb(item.Name, item.State, nil)
						skippedCount++
						continue
					}

					if item.State == Runnable && (!item.CausesRequired || e.isCausesDone(item.Causes...)) {
						item.State = Running
						e.onChangesCb(item.Name, item.State, nil)

						go func(ctx context.Context, item *StageMeta) {
							execFnCtx := context.WithValue(ctx, ContextKeyStageName, item.Name)

							if resErr := item.Fn(execFnCtx); resErr != nil {
								item.Err = resErr
							}

							item.State = Done
							e.onChangesCb(item.Name, item.State, item.Err)

							doneCh <- item.Name
						}(ctx, item)
					}
				}

				if skippedCount > 0 {
					go func() {
						allSkippedCh <- struct{}{}
					}()
					continue
				}

				if allDone {
					break ExecLoop
				}
			}
		}

		if e.finalCb != nil {
			execFnCtx := context.WithValue(ctx, ContextKeyStageName, Final)
			_ = e.finalCb(execFnCtx)
		}

		close(execNextCh)
		close(doneCh)
		close(allSkippedCh)

		e.doneFlag = true
		e.allJobsDoneChan <- struct{}{}
	}()

	execNextCh <- struct{}{} // initial push running

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
	e.RLock()
	defer e.RUnlock()

	return e.doneFlag
}

func (e *executorImpl) isCausesDone(causes ...StageName) bool {
	e.RLock()
	defer e.RUnlock()

	for _, s := range causes {
		if _, exists := e.causesDone[s]; !exists {
			return false
		}
	}
	return true
}

func (e *executorImpl) isAnyCausesFailedOrSkipped(causes ...StageName) bool {
	e.RLock()
	defer e.RUnlock()

	for _, s := range causes {
		for _, item := range e.registered {
			if item.Name != s {
				continue
			}

			if (item.State == Done && item.Err != nil) || item.State == Skipped {
				return true
			}
		}
	}
	return false
}
