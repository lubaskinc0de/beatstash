package application

import "errors"

var ErrUserNotFound = errors.New("user not found")
var ErrTrackNotFound = errors.New("track not found")
var ErrNotAuthenticated = errors.New("access denied")
var ErrCapabilityNotSupported = errors.New("provider capability not supported")

type FailureReason string

const (
	ReasonUnsupportedFormat FailureReason = "unsupported_format"
	ReasonCorruptFile       FailureReason = "corrupt_file"
	ReasonFetchFailed       FailureReason = "fetch_failed"
	ReasonInternal          FailureReason = "internal"
)

// PermanentError is an Ingest failure that retrying cannot fix.
type PermanentError struct {
	Reason FailureReason
	Err    error
}

func (e *PermanentError) Error() string {
	return string(e.Reason) + ": " + e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

func Permanent(reason FailureReason, err error) error {
	return &PermanentError{Reason: reason, Err: err}
}
