package asyncqu

import "errors"

var (
	ErrStageShouldNotWaitForItself = errors.New("stage should not wait for itself")
	ErrStageWaitForUnknown         = errors.New("stage wait for unknown")
	ErrEndStageIsNotSpecified      = errors.New("end stage is not specifier")
)
