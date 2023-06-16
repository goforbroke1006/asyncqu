package asyncqu

type StepName string

type JobState string

type JobItem struct {
	StepName StepName
	Fn       AsyncJobCallFn
	State    JobState
	Causes   []StepName
	Err      error
}
