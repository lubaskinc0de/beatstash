package providers

import (
	"errors"

	"github.com/lubaskinc0de/navidrome-tg/internal/domain/ingest"
)

var (
	ErrUnauthorized           = errors.New("provider does not accept the token")
	ErrNoSubscription         = errors.New("provider account has no subscription")
	ErrCapabilityNotSupported = errors.New("provider capability not supported")
)

// PermanentError is an Ingest failure that retrying cannot fix.
type PermanentError struct {
	Reason ingest.FailureReason
	Err    error
}

func (e *PermanentError) Error() string {
	return string(e.Reason) + ": " + e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

func Permanent(reason ingest.FailureReason, err error) error {
	return &PermanentError{Reason: reason, Err: err}
}
