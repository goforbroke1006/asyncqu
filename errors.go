package asyncqu

import "errors"

var (
	ErrExecutorWasNotStarted = errors.New("executor was not started")
	ErrEndStepIsNotSpecified = errors.New("end step is not specifier")
)
