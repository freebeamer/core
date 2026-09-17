package telemetry

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSpoolAppendAndPending(t *testing.T) {
	spool := &Spool{Path: filepath.Join(t.TempDir(), "buffer.jsonl")}
	for _, id := range []string{"a", "b", "c"} {
		if err := spool.Append(NewSample(id, fixedTime(0), nil)); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := spool.Pending()
	if err != nil {
		t.Fatal(err)
	}
	if pending != 3 {
		t.Fatalf("Pending = %d, want 3", pending)
	}
}

func TestSpoolOverflowDropCountsAndKeepsRecording(t *testing.T) {
	path := filepath.Join(t.TempDir(), "buffer.jsonl")
	spool := &Spool{Path: path, MaxBytes: 1, OnFull: OverflowDrop}
	if err := spool.Append(NewSample("a", fixedTime(0), nil)); err != nil {
		t.Fatal(err)
	}
	if dropped := spool.DroppedCount(); dropped != 1 {
		t.Fatalf("DroppedCount = %d, want 1", dropped)
	}
	pending, err := spool.Pending()
	if err != nil {
		t.Fatal(err)
	}
	if pending != 0 {
		t.Fatalf("Pending = %d, want 0 (dropped sample should not be written)", pending)
	}
}

func TestSpoolOverflowStopRefusesWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "buffer.jsonl")
	spool := &Spool{Path: path, MaxBytes: 1, OnFull: OverflowStop}
	err := spool.Append(NewSample("a", fixedTime(0), nil))
	if !errors.Is(err, ErrSpoolFull) {
		t.Fatalf("Append = %v, want ErrSpoolFull", err)
	}
}

func TestSpoolDrainDeliversInOrderAndEmpties(t *testing.T) {
	spool := &Spool{Path: filepath.Join(t.TempDir(), "buffer.jsonl")}
	for _, id := range []string{"a", "b", "c"} {
		if err := spool.Append(NewSample(id, fixedTime(0), nil)); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	delivered, remaining, err := spool.Drain(context.Background(), func(s Sample) error {
		got = append(got, s.DeviceID)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 3 || remaining != 0 {
		t.Fatalf("delivered=%d remaining=%d, want 3,0", delivered, remaining)
	}
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got = %v, want %v", got, want)
		}
	}
	if pending, _ := spool.Pending(); pending != 0 {
		t.Fatalf("Pending after full drain = %d, want 0", pending)
	}
}

func TestSpoolDrainStopsAtFirstFailureAndPreservesRest(t *testing.T) {
	spool := &Spool{Path: filepath.Join(t.TempDir(), "buffer.jsonl")}
	for _, id := range []string{"a", "b", "c"} {
		if err := spool.Append(NewSample(id, fixedTime(0), nil)); err != nil {
			t.Fatal(err)
		}
	}
	calls := 0
	_, remaining, err := spool.Drain(context.Background(), func(s Sample) error {
		calls++
		if calls == 2 {
			return errors.New("boom")
		}
		return nil
	})
	if err == nil {
		t.Fatal("Drain: want error, got nil")
	}
	if remaining != 2 {
		t.Fatalf("remaining = %d, want 2 (b and c retained)", remaining)
	}
	pending, _ := spool.Pending()
	if pending != 2 {
		t.Fatalf("Pending = %d, want 2", pending)
	}
}

func TestSpoolDrainPreservesUnreadableRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "buffer.jsonl")
	if err := os.WriteFile(path, []byte("{invalid json}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	spool := &Spool{Path: path}
	_, _, err := spool.Drain(context.Background(), func(Sample) error {
		t.Fatal("deliver should not be called for an unreadable record")
		return nil
	})
	if err == nil {
		t.Fatal("Drain: want error for unreadable record")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{invalid json}\n" {
		t.Fatalf("buffer changed: %q", data)
	}
}

// TestSpoolDrainPreservesConcurrentAppend is the direct regression test for
// the concurrency bug fixed in this phase: a Drain in flight (deliver is
// slow) must not lose a sample appended by another goroutine while it's
// running, even though Drain's rewrite step happens after deliver returns.
func TestSpoolDrainPreservesConcurrentAppend(t *testing.T) {
	spool := &Spool{Path: filepath.Join(t.TempDir(), "buffer.jsonl")}
	if err := spool.Append(NewSample("a", fixedTime(0), nil)); err != nil {
		t.Fatal(err)
	}

	release := make(chan struct{})
	var appended sync.WaitGroup
	appended.Add(1)

	go func() {
		defer appended.Done()
		<-release
		if err := spool.Append(NewSample("appended-during-drain", fixedTime(1), nil)); err != nil {
			t.Error(err)
		}
	}()

	delivered, remaining, err := spool.Drain(context.Background(), func(s Sample) error {
		close(release)
		appended.Wait() // ensure the concurrent append has landed before Drain's rewrite runs
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if delivered != 1 {
		t.Fatalf("delivered = %d, want 1", delivered)
	}
	if remaining != 1 {
		t.Fatalf("remaining = %d, want 1 (the concurrently appended sample must survive)", remaining)
	}

	pending, err := spool.Pending()
	if err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("Pending after drain = %d, want 1", pending)
	}

	var got string
	if _, _, err := spool.Drain(context.Background(), func(s Sample) error {
		got = s.DeviceID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got != "appended-during-drain" {
		t.Fatalf("surviving sample = %q, want %q", got, "appended-during-drain")
	}
}
