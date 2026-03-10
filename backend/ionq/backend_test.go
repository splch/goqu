package ionq

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/splch/qgo/backend"
	"github.com/splch/qgo/circuit/builder"
)

func TestSubmitAndResult(t *testing.T) {
	// Mock IonQ server.
	var submitted atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header.
		if r.Header.Get("Authorization") != "apiKey test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch {
		case r.Method == "POST" && r.URL.Path == "/jobs":
			// Verify request body.
			var req ionqJobRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Input.Qubits != 2 {
				t.Errorf("submitted qubits = %d, want 2", req.Input.Qubits)
			}
			if req.Input.Gateset != "qis" {
				t.Errorf("submitted gateset = %q, want %q", req.Input.Gateset, "qis")
			}
			submitted.Store(true)
			json.NewEncoder(w).Encode(ionqJobResponse{
				ID:     "job-123",
				Status: "submitted",
			})

		case r.Method == "GET" && r.URL.Path == "/jobs/job-123":
			json.NewEncoder(w).Encode(ionqStatusResponse{
				ID:     "job-123",
				Status: "completed",
				Qubits: 2,
			})

		case r.Method == "GET" && r.URL.Path == "/jobs/job-123/results/probabilities":
			json.NewEncoder(w).Encode(map[string]float64{
				"0": 0.5, // |00⟩
				"3": 0.5, // |11⟩
			})

		case r.Method == "PUT" && r.URL.Path == "/jobs/job-123/status/cancel":
			w.WriteHeader(http.StatusOK)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	b := New("test-key", WithDevice("simulator"), WithBaseURL(srv.URL))

	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	job, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   1000,
		Name:    "bell-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !submitted.Load() {
		t.Error("job was not submitted")
	}
	if job.ID != "job-123" {
		t.Errorf("job ID = %q, want %q", job.ID, "job-123")
	}

	// Check status.
	status, err := b.Status(context.Background(), "job-123")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateCompleted {
		t.Errorf("status = %s, want completed", status.State)
	}

	// Get results.
	result, err := b.Result(context.Background(), "job-123")
	if err != nil {
		t.Fatal(err)
	}
	if result.Probabilities["00"] != 0.5 {
		t.Errorf("P(00) = %v, want 0.5", result.Probabilities["00"])
	}
	if result.Probabilities["11"] != 0.5 {
		t.Errorf("P(11) = %v, want 0.5", result.Probabilities["11"])
	}
}

func TestSubmitAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(ionqAPIError{
			StatusCode: 401,
			Err:        "Unauthorized",
			Message:    "Invalid API key",
		})
	}))
	defer srv.Close()

	b := New("bad-key", WithBaseURL(srv.URL))
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("status code = %d, want 401", apiErr.StatusCode)
	}
}

func TestSubmitRetryOn429(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(ionqAPIError{
				StatusCode: 429,
				Err:        "Too Many Requests",
				Message:    "Rate limited",
			})
			return
		}
		json.NewEncoder(w).Encode(ionqJobResponse{
			ID:     "job-456",
			Status: "submitted",
		})
	}))
	defer srv.Close()

	b := New("key", WithBaseURL(srv.URL))
	c, _ := builder.New("test", 1).H(0).Build()
	job, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "job-456" {
		t.Errorf("job ID = %q, want %q", job.ID, "job-456")
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestResultNotCompleted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ionqStatusResponse{
			ID:     "job-789",
			Status: "running",
		})
	}))
	defer srv.Close()

	b := New("key", WithBaseURL(srv.URL))
	_, err := b.Result(context.Background(), "job-789")
	if err == nil {
		t.Fatal("expected error for non-completed job")
	}
}

func TestCancelJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	b := New("key", WithBaseURL(srv.URL))
	if err := b.Cancel(context.Background(), "job-123"); err != nil {
		t.Fatal(err)
	}
}

func TestNilCircuit(t *testing.T) {
	b := New("key")
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Shots: 100})
	if err == nil {
		t.Fatal("expected error for nil circuit")
	}
}

func TestZeroShots(t *testing.T) {
	b := New("key")
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Circuit: c})
	if err == nil {
		t.Fatal("expected error for zero shots")
	}
}

func TestDeviceTarget(t *testing.T) {
	tests := []struct {
		device string
		want   string
	}{
		{"simulator", "Simulator"},
		{"qpu.aria-1", "IonQ Aria"},
		{"qpu.aria-2", "IonQ Aria"},
		{"qpu.forte-1", "IonQ Forte"},
		{"qpu.forte-enterprise-1", "IonQ Forte"},
		{"unknown", "Simulator"},
	}
	for _, tt := range tests {
		got := deviceTarget(tt.device)
		if got.Name != tt.want {
			t.Errorf("deviceTarget(%q).Name = %q, want %q", tt.device, got.Name, tt.want)
		}
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		s    string
		want backend.JobState
	}{
		{"submitted", backend.StateSubmitted},
		{"ready", backend.StateReady},
		{"running", backend.StateRunning},
		{"completed", backend.StateCompleted},
		{"failed", backend.StateFailed},
		{"canceled", backend.StateCancelled},
		{"cancelled", backend.StateCancelled},
		{"unknown", backend.StateSubmitted},
	}
	for _, tt := range tests {
		if got := parseState(tt.s); got != tt.want {
			t.Errorf("parseState(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}
