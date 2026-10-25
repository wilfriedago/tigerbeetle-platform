package domain

import "time"

// TimeFromTB converts a TigerBeetle timestamp, nanoseconds since the Unix epoch, to a time in UTC.
// It is the only committed time the ledger knows.
func TimeFromTB(ts uint64) time.Time {
	return time.Unix(0, int64(ts)).UTC()
}
