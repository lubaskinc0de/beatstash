// RetryPolicy: how long a failed Ingest Job waits before each retry.

package ingest

import "time"

// RetryPolicy is a value object: how long to wait before each retry of a
// failed IngestJob.
type RetryPolicy struct {
	Delays []time.Duration
}

// Delay is how long to wait after the given failed attempt. It returns
// false when no retries are left.
func (p RetryPolicy) Delay(attempt int) (time.Duration, bool) {
	if attempt < 1 || attempt > len(p.Delays) {
		return 0, false
	}
	return p.Delays[attempt-1], true
}
