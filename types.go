package asyncqu

type StageName string

const (
	Start = StageName("start")
	Final = StageName("final")
	End   = StageName("end")
)

type State string

const (
	Runnable = State("runnable")
	Running  = State("running")
	Done     = State("done")
	Skipped  = State("skipped")
)

type StageMeta struct {
	Name   StageName
	Fn     StageFn
	State  State
	Causes []StageName
	Err    error
}
