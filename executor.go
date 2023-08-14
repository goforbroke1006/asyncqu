package asyncqu

import (
	"context"
	"fmt"
	"sync"
)

func New() Executor {
	return &executorImpl{
		stagesMap: map[StageName]*StageMeta{
			End: {
				Name:           End,
				Fn:             func(ctx context.Context) error { return nil },
				State:          Runnable,
				Causes:         []StageName{Start},
				CausesRequired: true,
			},
		},

		startedFlag: false,
		causesDone:  map[StageName]struct{}{},
		onChangesCb: func(name StageName, state State, err error) {},
		finalCb:     nil,
	}
}

type executorImpl struct {
	sync.RWMutex

	stagesMap map[StageName]*StageMeta

	startedFlag bool
	causesDone  map[StageName]struct{}
	onChangesCb OnChangedCb
	finalCb     StageFn
}

func (e *executorImpl) SetOnChanges(cb OnChangedCb) {
	e.Lock()
	defer e.Unlock()

	e.onChangesCb = cb
}

func (e *executorImpl) Append(stageName StageName, fn StageFn, causes ...StageName) {
	e.Lock()
	defer e.Unlock()

	delete(e.stagesMap, End)

	if _, exists := e.stagesMap[stageName]; exists {
		panic(fmt.Errorf("stage with name '%s' already exists", stageName))
	}

	for _, c := range causes {
		if c == stageName {
			panic(ErrStageShouldNotWaitForItself)
		}
		if c != Start {
			if _, exists := e.stagesMap[c]; !exists {
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
	e.stagesMap[stageName] = item
	e.onChangesCb(item.Name, item.State, nil)
}

func (e *executorImpl) SetEnd(causes ...StageName) {
	e.Lock()
	defer e.Unlock()

	e.stagesMap[End] = &StageMeta{
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

func (e *executorImpl) Run(ctx context.Context) error {
	if !e.hasEnd() {
		return ErrEndStageIsNotSpecified
	}

	e.Lock()
	e.startedFlag = true
	e.causesDone[Start] = struct{}{}
	e.Unlock()

	var (
		doneCh     = make(chan StageName)
		execNextCh = make(chan struct{}, 1)
	)

	execNextCh <- struct{}{} // initial push running

	go func(ctx context.Context) {
		// catch signal about finished stages
		for {
			select {
			case <-ctx.Done():
				return
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
	}(ctx)

ExecLoop:
	for {
		select {
		case <-ctx.Done():
			break ExecLoop
		case <-execNextCh:
			allDone := true
			skippedCount := 0

			for _, item := range e.stagesMap {
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

						if item.Fn != nil {
							if resErr := item.Fn(execFnCtx); resErr != nil {
								item.Err = resErr
							}
						}

						item.State = Done
						e.onChangesCb(item.Name, item.State, item.Err)

						select {
						case <-ctx.Done():
							// ok
						default:
							doneCh <- item.Name
						}

					}(ctx, item)
				}
			}

			if skippedCount > 0 {
				break ExecLoop
			}

			if allDone {
				break ExecLoop
			}
		}
	}

	// mark all skipped stages as Skipped
	for stageName := range e.stagesMap {
		if e.stagesMap[stageName].State != Runnable {
			continue
		}

		e.stagesMap[stageName].State = Skipped
		e.onChangesCb(e.stagesMap[stageName].Name, e.stagesMap[stageName].State, nil)
	}

	if e.finalCb != nil {
		execFnCtx := context.WithValue(ctx, ContextKeyStageName, Final)
		_ = e.finalCb(execFnCtx)
	}

	//close(execNextCh)
	//close(doneCh)
	//close(allSkippedCh)

	return nil
}

func (e *executorImpl) Errs() []error {
	errs := make([]error, 0)
	for _, item := range e.stagesMap {
		if item.Err != nil {
			errs = append(errs, item.Err)
		}
	}

	return errs
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
		for _, item := range e.stagesMap {
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

func (e *executorImpl) hasEnd() bool {
	e.RLock()
	defer e.RUnlock()

	for _, reg := range e.stagesMap {
		if reg.Name == End {
			return true
		}
	}

	return false
}
