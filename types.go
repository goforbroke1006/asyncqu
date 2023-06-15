package asyncqu

type StepName string

type JobState string

const (
	Runnable = JobState("runnable")
	Running  = JobState("running")
	Done     = JobState("done")
)

type JobItem struct {
	StepName StepName
	Fn       AsyncJobCallFn
	State    JobState
	Causes   []StepName
	Err      error
}
