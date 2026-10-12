package telemetry

import "time"

// Utilization returns percent of link speed from two octet counters.
// A counter decrease is treated as a reset and is not a utilization sample.
// speedBps is the interface rate in bits per second. A zero speed cannot be converted.
func Utilization(prevOctets, nextOctets uint64, elapsed time.Duration, speedBps uint64) (float64, bool) {
	if elapsed <= 0 || speedBps == 0 || nextOctets < prevOctets {
		return 0, false
	}
	delta := float64(nextOctets - prevOctets)
	util := delta * 8 / elapsed.Seconds() / float64(speedBps) * 100
	if util < 0 {
		return 0, false
	}
	return util, true
}
