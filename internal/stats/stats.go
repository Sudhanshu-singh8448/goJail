// Package stats provides global statistics tracking for goboxd.
package stats

import (
	"sync/atomic"
	"time"
)

// Stats tracks global execution statistics.
type Stats struct {
	InFlightJobs       int64
	JobsTotal          int64
	JobsFailedInternal int64
	LastInternalError  *time.Time
}

// Global statistics instance
var globalStats = &Stats{}

// IncrementInFlight increments the in-flight job counter.
func IncrementInFlight() {
	atomic.AddInt64(&globalStats.InFlightJobs, 1)
}

// DecrementInFlight decrements the in-flight job counter.
func DecrementInFlight() {
	atomic.AddInt64(&globalStats.InFlightJobs, -1)
}

// IncrementTotal increments the total jobs counter.
func IncrementTotal() {
	atomic.AddInt64(&globalStats.JobsTotal, 1)
}

// IncrementFailedInternal increments the internal error counter and records the timestamp.
func IncrementFailedInternal() {
	atomic.AddInt64(&globalStats.JobsFailedInternal, 1)
	now := time.Now()
	globalStats.LastInternalError = &now
}

// Get returns a snapshot of current statistics.
func Get() Stats {
	return Stats{
		InFlightJobs:       atomic.LoadInt64(&globalStats.InFlightJobs),
		JobsTotal:          atomic.LoadInt64(&globalStats.JobsTotal),
		JobsFailedInternal: atomic.LoadInt64(&globalStats.JobsFailedInternal),
		LastInternalError:  globalStats.LastInternalError,
	}
}
