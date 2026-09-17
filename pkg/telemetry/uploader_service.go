package telemetry

import (
	"context"
	"errors"
	"math/rand"
	"time"
)

// DefaultInitialBackoff and DefaultMaxBackoff bound the delay between
// delivery retries. "Bounded retries" here means a bounded backoff delay,
// not a bounded attempt count: an UploaderService never gives up on a
// spooled sample, since abandoning one would itself violate the
// never-silently-discard requirement in
// docs/live-data-and-vehicle-mapping-plan.md's Phase 3 section.
const (
	DefaultInitialBackoff = 1 * time.Second
	DefaultMaxBackoff     = 60 * time.Second
)

// negotiationCacheTTL bounds how long a cached capability-negotiation
// result is trusted before UploaderService re-checks the relay, so a
// backlog drain doesn't pay a capability round trip before every single
// queued sample the way Uploader.post's own per-call Negotiate would.
const negotiationCacheTTL = 5 * time.Minute

// idlePollInterval is how often Run checks the spool again after fully
// draining it, since there's no push notification from the spool-writer
// goroutine — simple polling at this interval is cheap and avoids adding a
// wake-up channel for a case that only matters within idlePollInterval of
// latency.
const idlePollInterval = 200 * time.Millisecond

// CatchupThreshold returns the minimum staleness (delivery time minus the
// sample's own acquisition Timestamp) at which a delivered sample counts as
// catch-up rather than live, for the given configured acquisition interval.
// Computed only from timestamps the device itself produced — never relay
// receipt time, per docs/telemetry-v2-contract.md's warning that network
// receipt can't prove freshness.
func CatchupThreshold(interval time.Duration) time.Duration {
	threshold := 2 * interval
	if threshold < 2*time.Second {
		threshold = 2 * time.Second
	}
	return threshold
}

// UploaderService continuously drains a Spool over HTTP, using an embedded
// Uploader's low-level postValidated/Negotiate primitives plus its own
// cached negotiation and bounded-delay retry/backoff, and publishes
// progress to a Status. Run owns exactly one goroutine's worth of work; it
// is not safe to call Run concurrently on the same UploaderService.
type UploaderService struct {
	Uploader *Uploader
	Spool    *Spool
	Status   *Status
	// Interval is the configured acquisition interval, used only to
	// compute CatchupThreshold.
	Interval time.Duration

	negotiatedVersion int
	negotiatedAt      time.Time
}

// Run drains Spool until ctx is cancelled, applying retry/backoff on
// delivery failure and reporting progress via Status. It returns once ctx
// is done — including while backed off or idle — after which Status
// reports UploadStopped.
//
// Cancellation does not abort an HTTP request already in flight (postValidated
// builds its own bounded timeout internally, not derived from ctx — the
// same limitation Uploader.post already has); Run only guarantees it will
// not *start* a new request after ctx is cancelled, and any request already
// running completes or times out on its own (bounded by Uploader.Timeout,
// 5s by default) before Run actually returns.
func (svc *UploaderService) Run(ctx context.Context) {
	backoff := DefaultInitialBackoff
	threshold := CatchupThreshold(svc.Interval)
	catchingUp := false
	catchupStart := 0

	for {
		if ctx.Err() != nil {
			svc.Status.SetUploadState(UploadStopped)
			return
		}

		startPending, _ := svc.Spool.Pending()
		svc.Status.SetSpool(startPending, svc.Spool.DroppedCount())
		svc.Status.SetUploadState(UploadUploading)

		_, _, err := svc.Spool.Drain(ctx, func(sample Sample) error {
			deliveredAt, derr := svc.deliver(sample)
			if derr != nil {
				return derr
			}
			catchup := deliveredAt.Sub(sample.Timestamp) > threshold
			if catchup && !catchingUp {
				catchingUp = true
				catchupStart = startPending
			}
			svc.Status.RecordDelivered(deliveredAt, catchup)
			return nil
		})

		newPending, _ := svc.Spool.Pending()
		svc.Status.SetSpool(newPending, svc.Spool.DroppedCount())

		if catchingUp {
			progress := 1.0
			if catchupStart > 0 {
				progress = 1 - float64(newPending)/float64(catchupStart)
				if progress < 0 {
					progress = 0
				}
			}
			svc.Status.SetCatchup(newPending > 0, progress, newPending)
			if newPending == 0 {
				catchingUp = false
			}
		} else {
			svc.Status.SetCatchup(false, 0, newPending)
		}

		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				svc.Status.SetUploadState(UploadStopped)
				return
			}
			svc.Status.SetError(err.Error())
			svc.Status.SetUploadState(UploadBackingOff)
			if !SleepWithJitter(ctx, backoff) {
				svc.Status.SetUploadState(UploadStopped)
				return
			}
			backoff = NextBackoff(backoff)
			continue
		}

		svc.Status.SetError("")
		backoff = DefaultInitialBackoff
		if newPending == 0 {
			svc.Status.SetUploadState(UploadIdle)
			select {
			case <-ctx.Done():
				svc.Status.SetUploadState(UploadStopped)
				return
			case <-time.After(idlePollInterval):
			}
		}
	}
}

func (svc *UploaderService) deliver(sample Sample) (time.Time, error) {
	if err := sample.Validate(); err != nil {
		return time.Time{}, err
	}
	if err := svc.negotiateIfNeeded(sample); err != nil {
		return time.Time{}, err
	}
	if err := svc.Uploader.postValidated(sample); err != nil {
		return time.Time{}, err
	}
	return time.Now(), nil
}

func (svc *UploaderService) negotiateIfNeeded(sample Sample) error {
	if sample.Version != ContractVersion {
		return nil // legacy samples: postValidated doesn't negotiate either.
	}
	if svc.negotiatedVersion == ContractVersion && time.Since(svc.negotiatedAt) < negotiationCacheTTL {
		return nil
	}
	version, err := svc.Uploader.Negotiate()
	if err != nil {
		return err
	}
	svc.negotiatedVersion = version
	svc.negotiatedAt = time.Now()
	if version != ContractVersion {
		return errors.New("telemetry: relay cannot preserve v2 sample identity")
	}
	return nil
}

// SleepWithJitter waits for d plus/minus 20% jitter, or until ctx is
// cancelled, whichever comes first — completed reports which happened.
// Jitter avoids synchronized retry storms across many clients backing off
// against the same relay outage at once. Exported so every reconnect/retry
// loop in this project shares one backoff implementation rather than each
// growing its own copy of the same jitter math — see internal/desktop's
// live-data relay consumer for the other caller.
func SleepWithJitter(ctx context.Context, d time.Duration) (completed bool) {
	jitter := time.Duration((rand.Float64()*0.4 - 0.2) * float64(d)) // +/-20%
	wait := d + jitter
	if wait <= 0 {
		wait = d
	}
	select {
	case <-ctx.Done():
		return false
	case <-time.After(wait):
		return true
	}
}

// NextBackoff doubles d, capped at DefaultMaxBackoff.
func NextBackoff(d time.Duration) time.Duration {
	next := d * 2
	if next > DefaultMaxBackoff {
		next = DefaultMaxBackoff
	}
	return next
}
