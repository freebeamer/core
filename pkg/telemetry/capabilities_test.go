package telemetry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestSharedCapabilities(t *testing.T) {
	data, err := os.ReadFile("../../testdata/public/telemetry/capabilities.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name    string
		Status  int
		Body    json.RawMessage
		Version int
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, test := range cases {
		t.Run(test.Name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != "/prefix/v1/capabilities" || r.Header.Get("Authorization") != "Bearer key" {
					t.Errorf("bad capability request: %s %s", r.Method, r.URL)
				}
				w.WriteHeader(test.Status)
				w.Write(test.Body)
			}))
			defer server.Close()
			u := Uploader{Endpoint: server.URL + "/prefix/v1/telemetry", APIKey: "key"}
			version, err := u.Negotiate()
			if test.Version == 0 {
				if err == nil {
					t.Fatal("accepted unknown capabilities")
				}
			} else if err != nil || version != test.Version {
				t.Fatalf("got %d, %v", version, err)
			}
		})
	}
}

func TestV2BufferRetryPreservesIdentityAcrossRestart(t *testing.T) {
	var requests []Sample
	compatible := false
	failPost := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if !compatible {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(RelayCapabilities())
			return
		}
		var sample Sample
		if err := json.NewDecoder(r.Body).Decode(&sample); err != nil {
			t.Error(err)
		}
		requests = append(requests, sample)
		if failPost {
			w.WriteHeader(503)
		} else {
			w.WriteHeader(202)
		}
	}))
	defer server.Close()
	data, err := os.ReadFile("../../testdata/public/telemetry/v2.json")
	if err != nil {
		t.Fatal(err)
	}
	var original Sample
	json.Unmarshal(data, &original)
	path := filepath.Join(t.TempDir(), "buffer.jsonl")
	u := Uploader{Endpoint: server.URL + "/v1/telemetry", BufferPath: path}
	if delivered, err := u.Send(original); err != nil || delivered {
		t.Fatalf("legacy destination: %v %v", delivered, err)
	}
	if len(requests) != 0 {
		t.Fatal("v2 posted to legacy relay")
	}
	compatible = true
	if err := u.FlushBuffer(); err == nil {
		t.Fatal("503 did not preserve buffer")
	}
	failPost = false
	restarted := Uploader{Endpoint: u.Endpoint, BufferPath: path}
	if err := restarted.FlushBuffer(); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || !reflect.DeepEqual(requests[0], original) || !reflect.DeepEqual(requests[1], original) {
		t.Fatalf("identity changed: %+v", requests)
	}
	if empty, err := restarted.bufferEmpty(); err != nil || !empty {
		t.Fatalf("buffer not drained: %v", err)
	}
}

func TestCapabilityTimeoutAndUnreadableBuffer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	u := Uploader{Endpoint: server.URL + "/v1/telemetry", Timeout: 10 * time.Millisecond, BufferPath: filepath.Join(t.TempDir(), "buffer")}
	if _, err := u.Negotiate(); err == nil {
		t.Fatal("timeout silently fell back")
	}
	const original = "{invalid json}\n"
	if err := os.WriteFile(u.BufferPath, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := u.FlushBuffer(); err == nil {
		t.Fatal("unreadable record discarded")
	}
	data, _ := os.ReadFile(u.BufferPath)
	if string(data) != original {
		t.Fatal("buffer changed")
	}
}
