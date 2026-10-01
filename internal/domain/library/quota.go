// Quota, Usage and Capacity answer whether a Track fits: how much a Library
// may weigh, how much it weighs now, and how much all the Quotas promise
// against the disk.

package library

import (
	"errors"
	"fmt"
)

// Quota is a value object: the most all Tracks of a Library may weigh
// together, in bytes. The zero Quota is unlimited.
type Quota int64

const Unlimited Quota = 0

// ServerQuotas is a value object: the Default Quota and the Shared
// Library's Quota, as the config sets them or as they hold now.
type ServerQuotas struct {
	Default Quota
	Shared  Quota
}

// Usage is a value object: how much a Library weighs against the Quota in
// force, measured at one moment.
type Usage struct {
	Used  int64
	Quota Quota
}

// QuotaExceededError tells how full the Library was when a Track did not fit.
type QuotaExceededError struct {
	Usage Usage
}

// Capacity is a value object: how much all the Quotas together promise,
// against what the bot's Libraries take and what is free on the disk.
type Capacity struct {
	Promised Quota
	Used     int64
	Free     int64
}

var ErrQuotaNegative = errors.New("quota is negative")

// NewQuota takes zero bytes as unlimited.
func NewQuota(bytes int64) (Quota, error) {
	if bytes < 0 {
		return 0, ErrQuotaNegative
	}
	return Quota(bytes), nil
}

// Promise sums the Quotas; a single unlimited one makes the sum unlimited.
func Promise(quotas []Quota) Quota {
	var sum Quota
	for _, q := range quotas {
		if !q.Limited() {
			return Unlimited
		}
		sum += q
	}
	return sum
}

func (q Quota) Limited() bool {
	return q != Unlimited
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

// CapacityOf sums the Quotas in force of the bot's Libraries and their
// weight; Attached Libraries are not the bot's files. weights maps library
// ids to the sum of their Tracks' sizes, free is what the disk has left.
func CapacityOf(libs []Library, server ServerQuotas, weights map[uint]int64, free int64) Capacity {
	capacity := Capacity{Free: free}
	promised := make([]Quota, 0, len(libs))
	for n := range libs {
		if libs[n].Attached() {
			continue
		}
		promised = append(promised, libs[n].quotaIn(server))
		capacity.Used += weights[libs[n].ID]
	}
	capacity.Promised = Promise(promised)
	return capacity
}

// Overcommitted reports whether the Quotas promise more than the disk can
// hold; an unlimited one always does. A Track kept in several Libraries
// counts in each, so the warning errs on the safe side.
func (c Capacity) Overcommitted() bool {
	return !c.Promised.Limited() || int64(c.Promised) > c.Used+c.Free
}
