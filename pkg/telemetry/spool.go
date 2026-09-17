package telemetry

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// OverflowPolicy controls what a Spool does once it reaches MaxBytes.
type OverflowPolicy string

const (
	// OverflowDrop drops the new sample, counts it, and keeps recording.
	OverflowDrop OverflowPolicy = "drop"
	// OverflowStop refuses the new sample with ErrSpoolFull; the caller
	// decides whether that means stopping acquisition entirely.
	OverflowStop OverflowPolicy = "stop"
)

// DefaultSpoolMaxBytes is used when Spool.MaxBytes is zero.
const DefaultSpoolMaxBytes = 50 * 1024 * 1024 // 50 MiB

// ErrSpoolFull is returned by Append when the spool is at MaxBytes and
// OnFull is OverflowStop.
var ErrSpoolFull = errors.New("telemetry: spool is full")

// Spool durably persists samples that haven't been delivered yet, one JSON
// object per line, oldest first. Unlike pkg/telemetry.Uploader's original
// unbounded buffer file (still used by Uploader.Send/FlushBuffer, kept
// as-is for backward compatibility — see uploader.go), Spool enforces a
// byte cap and serializes every append/drain through one mutex so a
// dedicated spool-writer goroutine and a draining uploader goroutine can
// safely share the same file concurrently. That concurrency is new in
// Phase 3: the original single-goroutine FlushBuffer's
// read-then-rewrite was only ever safe because nothing appended
// concurrently with a drain.
type Spool struct {
	// Path is the on-disk JSONL file.
	Path string
	// MaxBytes bounds the spool's on-disk size; DefaultSpoolMaxBytes if
	// zero.
	MaxBytes int64
	// OnFull selects overflow behavior once MaxBytes is reached;
	// OverflowDrop if empty.
	OnFull OverflowPolicy

	mu      sync.Mutex
	dropped uint64
}

func (s *Spool) maxBytes() int64 {
	if s.MaxBytes <= 0 {
		return DefaultSpoolMaxBytes
	}
	return s.MaxBytes
}

func (s *Spool) policy() OverflowPolicy {
	if s.OnFull == "" {
		return OverflowDrop
	}
	return s.OnFull
}

// Append durably writes sample to the spool. If the spool is already at
// MaxBytes, behavior follows OnFull: OverflowDrop counts the drop
// (DroppedCount) and returns nil without writing; OverflowStop returns
// ErrSpoolFull without writing, leaving the decision to stop recording to
// the caller. Either way, a full spool never silently loses the fact that
// it dropped something.
func (s *Spool) Append(sample Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(sample)
	if err != nil {
		return err
	}
	line := append(data, '\n')

	size, err := s.currentSizeLocked()
	if err != nil {
		return err
	}
	if size+int64(len(line)) > s.maxBytes() {
		if s.policy() == OverflowStop {
			return ErrSpoolFull
		}
		s.dropped++
		return nil
	}

	f, err := os.OpenFile(s.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(line)
	return err
}

// DroppedCount returns how many samples Append has dropped because the
// spool was full under OverflowDrop.
func (s *Spool) DroppedCount() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

// Pending returns the number of samples currently on disk.
func (s *Spool) Pending() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lines, err := s.readLinesLocked()
	if err != nil {
		return 0, err
	}
	return len(lines), nil
}

// Drain attempts to deliver every sample currently on disk, oldest first,
// via deliver. It stops at the first delivery failure, the first
// unreadable/malformed line, or ctx cancellation — mirroring
// Uploader.FlushBuffer's contract: never discard an unreadable record,
// preserve ordering. deliver is called without holding Spool's internal
// lock, so a slow/blocking deliver (e.g. an HTTP call) never blocks a
// concurrent Append; only the file-read and the final file-rewrite are
// serialized. Because Append only ever adds at the end of the file, any
// sample appended while a Drain call is in flight survives the rewrite —
// the rewrite step re-reads the file's current tail rather than reusing
// the stale snapshot Drain started from.
func (s *Spool) Drain(ctx context.Context, deliver func(Sample) error) (delivered, remaining int, err error) {
	s.mu.Lock()
	lines, err := s.readLinesLocked()
	s.mu.Unlock()
	if err != nil || len(lines) == 0 {
		return 0, 0, err
	}

	done := 0
	for done < len(lines) {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		default:
		}
		if err != nil {
			break
		}
		var sample Sample
		if unmarshalErr := json.Unmarshal([]byte(lines[done]), &sample); unmarshalErr != nil {
			err = fmt.Errorf("telemetry: spool: unreadable record: %w", unmarshalErr)
			break
		}
		if deliverErr := deliver(sample); deliverErr != nil {
			err = deliverErr
			break
		}
		done++
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	current, readErr := s.readLinesLocked()
	if readErr != nil {
		return done, 0, readErr
	}
	if done > len(current) {
		done = len(current) // defensive; the file only ever grows between reads here
	}
	if rewriteErr := s.rewriteLocked(current[done:]); rewriteErr != nil {
		return done, len(current) - done, rewriteErr
	}
	return done, len(current) - done, err
}

func (s *Spool) currentSizeLocked() (int64, error) {
	info, err := os.Stat(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (s *Spool) readLinesLocked() ([]string, error) {
	f, err := os.Open(s.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func (s *Spool) rewriteLocked(lines []string) error {
	if len(lines) == 0 {
		err := os.Remove(s.Path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var buf bytes.Buffer
	for _, line := range lines {
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Path)
}
