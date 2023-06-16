package asyncqu

import "context"

func New() Executor {
	return &executorImpl{
		registered:      []*StageMeta{},
		startedFlag:     false,
		causesDone:      map[StageName]struct{}{},
		allJobsDoneChan: make(chan struct{}),
		doneFlag:        false,
		onChangesCb:     func(name StageName, state State, err error) {},
		finalCb:         nil,
	}
}

type executorImpl struct {
	registered      []*StageMeta
	startedFlag     bool
	causesDone      map[StageName]struct{}
	allJobsDoneChan chan struct{}
	doneFlag        bool
	onChangesCb     OnChangedCb
	finalCb         StageFn
}

func (e *executorImpl) SetOnChanges(cb OnChangedCb) {
	e.onChangesCb = cb
}

func (e *executorImpl) Append(stageName StageName, fn StageFn, clauses ...StageName) {
	e.registered = append(e.registered, &StageMeta{
		Name:           stageName,
		Fn:             fn,
		State:          Runnable,
		Causes:         clauses,
		CausesRequired: true,
	})
}

func (e *executorImpl) SetEnd(stageNames ...StageName) {
	e.registered = append(e.registered, &StageMeta{
		Name:           End,
		Fn:             func(ctx context.Context) error { return nil },
		State:          Runnable,
		Causes:         stageNames,
		CausesRequired: true,
	})
}

func (e *executorImpl) SetFinal(job StageFn) {
	e.finalCb = job
}

func (e *executorImpl) hasEnd() bool {
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

	e.startedFlag = true
	e.causesDone[Start] = struct{}{}

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
				e.causesDone[stageName] = struct{}{}
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

					e.onChangesCb(item.Name, item.State, nil)

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
	return e.doneFlag
}

func (e *executorImpl) isCausesDone(causes ...StageName) bool {
	for _, s := range causes {
		if _, exists := e.causesDone[s]; !exists {
			return false
		}
	}
	return true
}

func (e *executorImpl) isAnyCausesFailedOrSkipped(causes ...StageName) bool {
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
