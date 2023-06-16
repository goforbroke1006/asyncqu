package asyncqu

const (
	Start = StepName("start")
	End   = StepName("end")
)

const (
	Runnable = JobState("runnable")
	Running  = JobState("running")
	Done     = JobState("done")
)
