package telemetry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestUploaderServiceDeliversSpooledSamples(t *testing.T) {
	var mu sync.Mutex
	var received []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sample Sample
		json.NewDecoder(r.Body).Decode(&sample)
		mu.Lock()
		received = append(received, sample.DeviceID)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	spool := &Spool{Path: filepath.Join(t.TempDir(), "buffer.jsonl")}
	for _, id := range []string{"a", "b", "c"} {
		if err := spool.Append(NewSample(id, time.Now(), nil)); err != nil {
			t.Fatal(err)
		}
	}

	svc := &UploaderService{
		Uploader: &Uploader{Endpoint: server.URL},
		Spool:    spool,
		Status:   NewStatus(),
		Interval: 100 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() { svc.Run(ctx); close(done) }()

	waitForCondition(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(received) == 3
	})
	cancel()
	<-done

	mu.Lock()
	defer mu.Unlock()
	want := []string{"a", "b", "c"}
	if len(received) != len(want) {
		t.Fatalf("received = %v, want %v", received, want)
	}
	for i := range want {
		if received[i] != want[i] {
			t.Fatalf("received = %v, want %v", received, want)
		}
	}
}

func TestUploaderServiceRecoversAfterBackoff(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		var sample Sample
		json.NewDecoder(r.Body).Decode(&sample)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	spool := &Spool{Path: filepath.Join(t.TempDir(), "buffer.jsonl")}
	if err := spool.Append(NewSample("a", time.Now(), nil)); err != nil {
		t.Fatal(err)
	}

	svc := &UploaderService{
		Uploader: &Uploader{Endpoint: server.URL},
		Spool:    spool,
		Status:   NewStatus(),
		Interval: 50 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() { svc.Run(ctx); close(done) }()

	waitForCondition(t, 5*time.Second, func() bool {
		pending, _ := spool.Pending()
		return pending == 0
	})
	cancel()
	<-done

	if n := atomic.LoadInt32(&calls); n < 2 {
		t.Fatalf("calls = %d, want at least 2 (one failure, one successful retry)", n)
	}
}

func TestUploaderServiceStopsPromptlyOnCancellation(t *testing.T) {
	spool := &Spool{Path: filepath.Join(t.TempDir(), "buffer.jsonl")}
	svc := &UploaderService{
		Uploader: &Uploader{Endpoint: ""}, // offline
		Spool:    spool,
		Status:   NewStatus(),
		Interval: 100 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() { svc.Run(ctx); close(done) }()

	waitForCondition(t, time.Second, func() bool {
		return svc.Status.Snapshot().UploadState == UploadIdle
	})
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return promptly after cancellation")
	}
	if state := svc.Status.Snapshot().UploadState; state != UploadStopped {
		t.Fatalf("UploadState after cancellation = %q, want %q", state, UploadStopped)
	}
}

func TestUploaderServiceReportsCatchupProgress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	spool := &Spool{Path: filepath.Join(t.TempDir(), "buffer.jsonl")}
	old := time.Now().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		if err := spool.Append(NewSample("old", old, nil)); err != nil {
			t.Fatal(err)
		}
	}

	svc := &UploaderService{
		Uploader: &Uploader{Endpoint: server.URL},
		Spool:    spool,
		Status:   NewStatus(),
		Interval: 100 * time.Millisecond,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() { svc.Run(ctx); close(done) }()

	waitForCondition(t, 2*time.Second, func() bool {
		pending, _ := spool.Pending()
		return pending == 0
	})
	cancel()
	<-done

	snap := svc.Status.Snapshot()
	if snap.CatchupProgress != 1 {
		t.Fatalf("CatchupProgress after full drain = %v, want 1", snap.CatchupProgress)
	}
	if snap.CatchupInProgress {
		t.Fatal("CatchupInProgress should be false once the backlog is fully drained")
	}
	// A stale sample delivered well after its own timestamp must not be
	// reported as the "last fresh" one.
	if !snap.LastFreshSampleAt.IsZero() {
		t.Fatalf("LastFreshSampleAt = %v, want zero (every delivered sample here was catch-up, not live)", snap.LastFreshSampleAt)
	}
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !cond() {
		t.Fatal("condition not met within timeout")
	}
}
