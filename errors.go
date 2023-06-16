package asyncqu

import "errors"

var (
	ErrExecutorWasNotStarted  = errors.New("executor was not started")
	ErrEndStageIsNotSpecified = errors.New("end stage is not specifier")
)
