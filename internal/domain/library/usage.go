package library

import "fmt"

// Usage is a value object: how much a Library weighs against the Quota in
// force, measured at one moment. It goes stale once the Library's lock is
// let go.
type Usage struct {
	Used  int64
	Quota Quota
}

// QuotaExceededError tells how full the Library was when a Track did not fit.
type QuotaExceededError struct {
	Usage Usage
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("quota exceeded: %d of %d bytes used", e.Usage.Used, e.Usage.Quota)
}

// Admit returns a *QuotaExceededError unless the Library can grow by
// growth bytes. A Track that replaces a Duplicate grows it by the
// difference in size, which may be negative: that always fits.
func (u Usage) Admit(growth int64) error {
	if !u.Quota.Limited() || growth <= 0 || u.Used+growth <= int64(u.Quota) {
		return nil
	}
	return &QuotaExceededError{Usage: u}
}

// Take admits growth and counts it as used, for several Tracks that must
// fit together.
func (u *Usage) Take(growth int64) error {
	if err := u.Admit(growth); err != nil {
		return err
	}
	u.Used += growth
	return nil
}

// Fit counts how many Tracks of the sizes, taken in order, fit together.
// A full Library takes none, even of size zero.
func (u Usage) Fit(sizes []int64) int {
	for n, size := range sizes {
		if u.Full() || u.Take(size) != nil {
			return n
		}
	}
	return len(sizes)
}

// RequireRoom returns a *QuotaExceededError when the Library is full: then
// no Track fits, whatever its size.
func (u Usage) RequireRoom() error {
	if u.Full() {
		return &QuotaExceededError{Usage: u}
	}
	return nil
}

// Full reports whether no Track fits any more.
func (u Usage) Full() bool {
	return u.Quota.Limited() && u.Used >= int64(u.Quota)
}

// Free is zero for a full Library and meaningless for an unlimited one.
func (u Usage) Free() int64 {
	return max(int64(u.Quota)-u.Used, 0)
}
