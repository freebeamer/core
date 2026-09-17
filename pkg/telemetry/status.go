package telemetry

import (
	"sync"
	"time"
)

// UploadState describes what an UploaderService is currently doing.
type UploadState string

const (
	UploadIdle       UploadState = "idle"       // spool empty, waiting for new samples
	UploadUploading  UploadState = "uploading"  // actively delivering
	UploadBackingOff UploadState = "backingOff" // a delivery failed; waiting before retrying
	UploadStopped    UploadState = "stopped"    // the service has shut down
)

// rateWindow bounds how many recent acquisition timestamps are kept for
// computing a measured (not configured) acquisition rate.
const rateWindow = 20

// StatusSnapshot is a point-in-time, read-only view of an acquisition +
// spool + upload pipeline's state, safe to read from any goroutine — the
// data behind command_mhd.go's periodic stderr status line and any future
// caller (e.g. a desktop/mobile status surface) that wants the same
// information.
type StatusSnapshot struct {
	QueueDepth   int
	QueueDropped uint64

	SpoolPending int
	SpoolDropped uint64

	UploadState       UploadState
	LastFreshSampleAt time.Time // zero if nothing has been delivered live yet
	LastError         string

	// AcquisitionRateHz is measured from actual inter-sample timestamps,
	// never derived from the configured --interval — see
	// docs/live-data-and-vehicle-mapping-plan.md Phase 3's "record actual
	// achievable rates" acceptance requirement.
	AcquisitionRateHz float64

	CatchupInProgress bool
	CatchupProgress   float64 // 0..1; meaningful only if CatchupInProgress
	CatchupPending    int
}

// Status is a thread-safe holder for the current StatusSnapshot, written by
// the acquisition, spool-writer, and uploader goroutines and read by the
// periodic status reporter.
type Status struct {
	mu       sync.Mutex
	snapshot StatusSnapshot

	rateMu    sync.Mutex
	rateTimes []time.Time
}

// NewStatus returns a Status with a zero StatusSnapshot.
func NewStatus() *Status { return &Status{} }

// Snapshot returns the current state.
func (s *Status) Snapshot() StatusSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshot
}

func (s *Status) update(fn func(*StatusSnapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.snapshot)
}

// RecordAcquired records that a sample was just read from the adapter, at
// t, updating the measured acquisition rate.
func (s *Status) RecordAcquired(t time.Time) {
	s.rateMu.Lock()
	s.rateTimes = append(s.rateTimes, t)
	if len(s.rateTimes) > rateWindow {
		s.rateTimes = s.rateTimes[len(s.rateTimes)-rateWindow:]
	}
	var hz float64
	if len(s.rateTimes) >= 2 {
		span := s.rateTimes[len(s.rateTimes)-1].Sub(s.rateTimes[0]).Seconds()
		if span > 0 {
			hz = float64(len(s.rateTimes)-1) / span
		}
	}
	s.rateMu.Unlock()
	s.update(func(snap *StatusSnapshot) { snap.AcquisitionRateHz = hz })
}

// SetQueue records the acquisition queue's current depth and cumulative
// overflow count.
func (s *Status) SetQueue(depth int, dropped uint64) {
	s.update(func(snap *StatusSnapshot) { snap.QueueDepth = depth; snap.QueueDropped = dropped })
}

// SetSpool records the durable spool's current pending count and
// cumulative overflow-drop count.
func (s *Status) SetSpool(pending int, dropped uint64) {
	s.update(func(snap *StatusSnapshot) { snap.SpoolPending = pending; snap.SpoolDropped = dropped })
}

// SetUploadState records the uploader's current activity.
func (s *Status) SetUploadState(state UploadState) {
	s.update(func(snap *StatusSnapshot) { snap.UploadState = state })
}

// SetError records the most recent delivery error's message ("" to clear).
func (s *Status) SetError(msg string) {
	s.update(func(snap *StatusSnapshot) { snap.LastError = msg })
}

// RecordDelivered records that a sample was just delivered. Only a
// non-catch-up (live) delivery updates LastFreshSampleAt — a catch-up
// delivery is, by definition, not evidence anything is currently fresh.
func (s *Status) RecordDelivered(deliveredAt time.Time, catchup bool) {
	s.update(func(snap *StatusSnapshot) {
		if !catchup {
			snap.LastFreshSampleAt = deliveredAt
		}
	})
}

// SetCatchup records catch-up state: whether one is in progress, its
// progress against the backlog size observed when it was first detected,
// and the raw pending count (which "50%" alone can't distinguish between a
// backlog of 3 and a backlog of 30,000).
func (s *Status) SetCatchup(inProgress bool, progress float64, pending int) {
	s.update(func(snap *StatusSnapshot) {
		snap.CatchupInProgress = inProgress
		snap.CatchupProgress = progress
		snap.CatchupPending = pending
	})
}
