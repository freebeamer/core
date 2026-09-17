package telemetry

import "time"

// fixedTime returns a deterministic, distinct timestamp for test sample n,
// so ordering assertions don't depend on wall-clock timing.
func fixedTime(n int) time.Time {
	return time.Date(2026, 9, 16, 12, 0, n, 0, time.UTC)
}
