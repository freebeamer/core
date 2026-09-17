package telemetry

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/freebeamer/core/pkg/mhd"
)

func TestNewSample(t *testing.T) {
	values := []mhd.Value{
		{Field: mhd.Field{Name: "RPM"}, Raw: 800, Value: 800},
		{Field: mhd.Field{Name: "LOAD"}, Raw: -50, Value: -0.5},
	}
	ts := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	sample := NewSample("device-1", ts, values)

	if sample.DeviceID != "device-1" {
		t.Fatalf("DeviceID = %q, want %q", sample.DeviceID, "device-1")
	}
	if !sample.Timestamp.Equal(ts) {
		t.Fatalf("Timestamp = %v, want %v", sample.Timestamp, ts)
	}
	if sample.Values["RPM"] != 800 || sample.Values["LOAD"] != -0.5 {
		t.Fatalf("Values = %+v", sample.Values)
	}
}

func bufferLineCount(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if scanner.Text() != "" {
			count++
		}
	}
	return count
}

func TestUploaderSendDeliversWhenReachable(t *testing.T) {
	var received []Sample
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sample Sample
		if err := json.NewDecoder(r.Body).Decode(&sample); err != nil {
			t.Errorf("server: decode: %v", err)
		}
		received = append(received, sample)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	bufferPath := filepath.Join(t.TempDir(), "buffer.jsonl")
	uploader := &Uploader{Endpoint: server.URL, BufferPath: bufferPath}

	sample := NewSample("device-1", time.Now(), nil)
	delivered, err := uploader.Send(sample)
	if err != nil {
		t.Fatal(err)
	}
	if !delivered {
		t.Fatal("Send: delivered = false, want true")
	}
	if len(received) != 1 || received[0].DeviceID != "device-1" {
		t.Fatalf("server received = %+v", received)
	}
	if n := bufferLineCount(t, bufferPath); n != 0 {
		t.Fatalf("buffer has %d line(s), want 0", n)
	}
}

func TestUploaderSendSetsAuthorizationHeaderWhenAPIKeySet(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uploader := &Uploader{
		Endpoint:   server.URL,
		APIKey:     "fh_live_test-key",
		BufferPath: filepath.Join(t.TempDir(), "buffer.jsonl"),
	}
	if _, err := uploader.Send(NewSample("device-1", time.Now(), nil)); err != nil {
		t.Fatal(err)
	}
	if want := "Bearer fh_live_test-key"; gotAuth != want {
		t.Fatalf("Authorization header = %q, want %q", gotAuth, want)
	}
}

func TestUploaderSendOmitsAuthorizationHeaderWhenAPIKeyUnset(t *testing.T) {
	var sawAuthHeader bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, sawAuthHeader = r.Header["Authorization"]
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	uploader := &Uploader{Endpoint: server.URL, BufferPath: filepath.Join(t.TempDir(), "buffer.jsonl")}
	if _, err := uploader.Send(NewSample("device-1", time.Now(), nil)); err != nil {
		t.Fatal(err)
	}
	if sawAuthHeader {
		t.Fatal("Authorization header present with no APIKey set")
	}
}

func TestUploaderSendBuffersWhenUnreachable(t *testing.T) {
	bufferPath := filepath.Join(t.TempDir(), "buffer.jsonl")
	uploader := &Uploader{
		Endpoint:   "http://127.0.0.1:1", // refused
		BufferPath: bufferPath,
		Timeout:    200 * time.Millisecond,
	}

	sample := NewSample("device-1", time.Now(), nil)
	delivered, err := uploader.Send(sample)
	if err != nil {
		t.Fatal(err)
	}
	if delivered {
		t.Fatal("Send: delivered = true, want false")
	}
	if n := bufferLineCount(t, bufferPath); n != 1 {
		t.Fatalf("buffer has %d line(s), want 1", n)
	}
}

func TestUploaderSendWithNoEndpointAlwaysBuffers(t *testing.T) {
	bufferPath := filepath.Join(t.TempDir(), "buffer.jsonl")
	uploader := &Uploader{BufferPath: bufferPath}

	delivered, err := uploader.Send(NewSample("device-1", time.Now(), nil))
	if err != nil {
		t.Fatal(err)
	}
	if delivered {
		t.Fatal("Send: delivered = true, want false")
	}
	if n := bufferLineCount(t, bufferPath); n != 1 {
		t.Fatalf("buffer has %d line(s), want 1", n)
	}
}

func TestUploaderFlushBufferDrainsInOrder(t *testing.T) {
	var received []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sample Sample
		json.NewDecoder(r.Body).Decode(&sample)
		received = append(received, sample.DeviceID)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	bufferPath := filepath.Join(t.TempDir(), "buffer.jsonl")
	uploader := &Uploader{Endpoint: server.URL, BufferPath: bufferPath}
	// Buffer three samples directly (as if written while unreachable).
	for _, id := range []string{"a", "b", "c"} {
		if err := uploader.buffer(NewSample(id, time.Now(), nil)); err != nil {
			t.Fatal(err)
		}
	}

	if err := uploader.FlushBuffer(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"a", "b", "c"}; !stringSlicesEqual(received, want) {
		t.Fatalf("received = %v, want %v", received, want)
	}
	if n := bufferLineCount(t, bufferPath); n != 0 {
		t.Fatalf("buffer has %d line(s), want 0", n)
	}
}

func TestUploaderFlushBufferStopsAtFirstFailure(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	bufferPath := filepath.Join(t.TempDir(), "buffer.jsonl")
	uploader := &Uploader{Endpoint: server.URL, BufferPath: bufferPath}
	for _, id := range []string{"a", "b", "c"} {
		if err := uploader.buffer(NewSample(id, time.Now(), nil)); err != nil {
			t.Fatal(err)
		}
	}

	if err := uploader.FlushBuffer(); err == nil {
		t.Fatal("FlushBuffer: want error, got nil")
	}
	if n := bufferLineCount(t, bufferPath); n != 2 {
		t.Fatalf("buffer has %d line(s), want 2 (b and c retained)", n)
	}
}

func TestUploaderSendDrainsBacklogBeforeNewSample(t *testing.T) {
	var received []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sample Sample
		json.NewDecoder(r.Body).Decode(&sample)
		received = append(received, sample.DeviceID)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	bufferPath := filepath.Join(t.TempDir(), "buffer.jsonl")
	uploader := &Uploader{Endpoint: server.URL, BufferPath: bufferPath}
	if err := uploader.buffer(NewSample("old", time.Now(), nil)); err != nil {
		t.Fatal(err)
	}

	delivered, err := uploader.Send(NewSample("new", time.Now(), nil))
	if err != nil {
		t.Fatal(err)
	}
	if !delivered {
		t.Fatal("Send: delivered = false, want true")
	}
	if want := []string{"old", "new"}; !stringSlicesEqual(received, want) {
		t.Fatalf("received = %v, want %v", received, want)
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
