package asyncqu

type StageName string

type State string

type StageMeta struct {
	Name           StageName
	Fn             StageFn
	State          State
	Causes         []StageName
	CausesRequired bool
	Err            error
}
