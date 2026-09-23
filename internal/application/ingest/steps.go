package ingest

type step string

const (
	stepSource step = "source"
	stepFetch  step = "fetch"
	stepStage  step = "stage"
	stepRemux  step = "remux"
	stepProbe  step = "probe"
	stepStore  step = "store"
)

type stepError struct {
	step step
	err  error
}

func (e *stepError) Error() string { return string(e.step) + ": " + e.err.Error() }
func (e *stepError) Unwrap() error { return e.err }

func wrapStep(s step, err error) error {
	if err == nil {
		return nil
	}
	return &stepError{step: s, err: err}
}
