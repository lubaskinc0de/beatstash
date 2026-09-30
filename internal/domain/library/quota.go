package library

import "errors"

// Quota is a value object: the most all Tracks of a Library may weigh
// together, in bytes. The zero Quota is unlimited.
type Quota int64

const Unlimited Quota = 0

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
