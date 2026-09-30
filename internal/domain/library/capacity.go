package library

// Capacity is a value object: how much all the Quotas together promise,
// against what the bot's Libraries take and what is free on the disk.
type Capacity struct {
	Promised Quota
	Used     int64
	Free     int64
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
