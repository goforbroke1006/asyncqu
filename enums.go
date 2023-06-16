package asyncqu

const (
	Start = StageName("start")
	Final = StageName("final")
	End   = StageName("end")
)

const (
	Runnable = State("runnable")
	Running  = State("running")
	Done     = State("done")
	Skipped  = State("skipped")
)
