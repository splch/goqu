package ibm

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

// newTestBackend creates a Backend wired to mock IAM and API servers.
func newTestBackend(t *testing.T, iamSrv, apiSrv *httptest.Server) *Backend {
	t.Helper()
	return New("test-api-key", "crn:v1:bluemix:public:quantum:us-east:a/123:::",
		WithBaseURL(apiSrv.URL),
		WithIAMURL(iamSrv.URL),
		WithDevice("ibm_brisbane"),
	)
}

// mockIAM returns an httptest.Server that always returns a valid token.
func mockIAM() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(iamTokenResponse{
			AccessToken: "test-bearer-token",
			ExpiresIn:   3600,
		})
	}))
}

func TestTokenExchange(t *testing.T) {
	var called atomic.Bool
	iamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)

		if r.Method != http.MethodPost {
			t.Errorf("IAM method = %s, want POST", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-www-form-urlencoded" {
			t.Errorf("IAM Content-Type = %q, want application/x-www-form-urlencoded", ct)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.FormValue("grant_type"); got != "urn:ibm:params:oauth:grant-type:apikey" {
			t.Errorf("grant_type = %q, want urn:ibm:params:oauth:grant-type:apikey", got)
		}
		if got := r.FormValue("apikey"); got != "test-api-key" {
			t.Errorf("apikey = %q, want test-api-key", got)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(iamTokenResponse{
			AccessToken: "fresh-token",
			ExpiresIn:   3600,
		})
	}))
	defer iamSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify bearer token was used.
		if got := r.Header.Get("Authorization"); got != "Bearer fresh-token" {
			t.Errorf("Authorization = %q, want Bearer fresh-token", got)
		}
		json.NewEncoder(w).Encode(ibmJobResponse{ID: "job-tok"})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, iamSrv, apiSrv)
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called.Load() {
		t.Error("IAM token endpoint was not called")
	}
}

func TestSubmitAndResult(t *testing.T) {
	iamSrv := mockIAM()
	defer iamSrv.Close()

	var submitted atomic.Bool
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify required IBM headers.
		if got := r.Header.Get("Authorization"); got != "Bearer test-bearer-token" {
			t.Errorf("Authorization = %q, want Bearer test-bearer-token", got)
		}
		if got := r.Header.Get("Service-CRN"); got != "crn:v1:bluemix:public:quantum:us-east:a/123:::" {
			t.Errorf("Service-CRN = %q", got)
		}
		if got := r.Header.Get("IBM-API-Version"); got != defaultAPIVersion {
			t.Errorf("IBM-API-Version = %q, want %q", got, defaultAPIVersion)
		}

		switch {
		case r.Method == "POST" && r.URL.Path == "/jobs":
			var req ibmJobRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.ProgramID != "sampler" {
				t.Errorf("program_id = %q, want sampler", req.ProgramID)
			}
			if req.Backend != "ibm_brisbane" {
				t.Errorf("backend = %q, want ibm_brisbane", req.Backend)
			}
			if req.Params.Version != 2 {
				t.Errorf("params.version = %d, want 2", req.Params.Version)
			}
			if len(req.Params.Pubs) != 1 || len(req.Params.Pubs[0]) != 1 {
				t.Errorf("pubs shape unexpected: %v", req.Params.Pubs)
			}
			// Verify the QASM string contains expected content.
			qasm := req.Params.Pubs[0][0]
			if len(qasm) == 0 {
				t.Error("empty QASM string in PUB")
			}
			submitted.Store(true)
			json.NewEncoder(w).Encode(ibmJobResponse{ID: "job-ibm-123"})

		case r.Method == "GET" && r.URL.Path == "/jobs/job-ibm-123":
			json.NewEncoder(w).Encode(ibmStatusResponse{
				ID:      "job-ibm-123",
				Status:  "Completed",
				Backend: "ibm_brisbane",
			})

		case r.Method == "GET" && r.URL.Path == "/jobs/job-ibm-123/results":
			json.NewEncoder(w).Encode(ibmResultResponse{
				Results: []ibmPubResult{{
					Data: ibmResultData{
						CRegSamples: map[string][][]int{
							"meas": {
								{0, 0}, // |00>
								{0, 0}, // |00>
								{1, 1}, // |11>
								{1, 1}, // |11>
							},
						},
					},
				}},
			})

		case r.Method == "POST" && r.URL.Path == "/jobs/job-ibm-123/cancel":
			w.WriteHeader(http.StatusOK)

		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, iamSrv, apiSrv)

	c, _ := builder.New("bell", 2).H(0).CNOT(0, 1).Build()
	job, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   4,
		Name:    "bell-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !submitted.Load() {
		t.Error("job was not submitted")
	}
	if job.ID != "job-ibm-123" {
		t.Errorf("job ID = %q, want %q", job.ID, "job-ibm-123")
	}
	if job.Backend != "ibm.ibm_brisbane" {
		t.Errorf("backend = %q, want ibm.ibm_brisbane", job.Backend)
	}

	// Check status.
	status, err := b.Status(context.Background(), "job-ibm-123")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateCompleted {
		t.Errorf("status = %s, want completed", status.State)
	}

	// Get results.
	result, err := b.Result(context.Background(), "job-ibm-123")
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts["00"] != 2 {
		t.Errorf("Counts[00] = %d, want 2", result.Counts["00"])
	}
	if result.Counts["11"] != 2 {
		t.Errorf("Counts[11] = %d, want 2", result.Counts["11"])
	}
}

func TestStatusAllStates(t *testing.T) {
	iamSrv := mockIAM()
	defer iamSrv.Close()

	tests := []struct {
		ibmStatus string
		want      backend.JobState
	}{
		{"Queued", backend.StateSubmitted},
		{"Running", backend.StateRunning},
		{"Completed", backend.StateCompleted},
		{"Failed", backend.StateFailed},
		{"Cancelled", backend.StateCancelled},
	}

	for _, tt := range tests {
		t.Run(tt.ibmStatus, func(t *testing.T) {
			apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(ibmStatusResponse{
					ID:     "job-state",
					Status: tt.ibmStatus,
				})
			}))
			defer apiSrv.Close()

			b := newTestBackend(t, iamSrv, apiSrv)
			status, err := b.Status(context.Background(), "job-state")
			if err != nil {
				t.Fatal(err)
			}
			if status.State != tt.want {
				t.Errorf("parseState(%q) = %v, want %v", tt.ibmStatus, status.State, tt.want)
			}
		})
	}
}

func TestStatusWithError(t *testing.T) {
	iamSrv := mockIAM()
	defer iamSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ibmStatusResponse{
			ID:     "job-fail",
			Status: "Failed",
			Error:  &ibmError{Message: "circuit too deep", Code: "CIRCUIT_ERROR"},
		})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, iamSrv, apiSrv)
	status, err := b.Status(context.Background(), "job-fail")
	if err != nil {
		t.Fatal(err)
	}
	if status.State != backend.StateFailed {
		t.Errorf("state = %v, want failed", status.State)
	}
	if status.Error != "circuit too deep" {
		t.Errorf("error = %q, want %q", status.Error, "circuit too deep")
	}
}

func TestResultNotCompleted(t *testing.T) {
	iamSrv := mockIAM()
	defer iamSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ibmStatusResponse{
			ID:     "job-running",
			Status: "Running",
		})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, iamSrv, apiSrv)
	_, err := b.Result(context.Background(), "job-running")
	if err == nil {
		t.Fatal("expected error for non-completed job")
	}
}

func TestCancelJob(t *testing.T) {
	iamSrv := mockIAM()
	defer iamSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/jobs/job-cancel/cancel" {
			t.Errorf("path = %s, want /jobs/job-cancel/cancel", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, iamSrv, apiSrv)
	if err := b.Cancel(context.Background(), "job-cancel"); err != nil {
		t.Fatal(err)
	}
}

func TestNilCircuit(t *testing.T) {
	b := New("key", "crn")
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Shots: 100})
	if err == nil {
		t.Fatal("expected error for nil circuit")
	}
}

func TestZeroShots(t *testing.T) {
	b := New("key", "crn")
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{Circuit: c})
	if err == nil {
		t.Fatal("expected error for zero shots")
	}
}

func TestRetryOn429(t *testing.T) {
	iamSrv := mockIAM()
	defer iamSrv.Close()

	var attempts atomic.Int32
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(ibmAPIError{
				Code:    "rate_limit",
				Message: "Rate limited",
			})
			return
		}
		json.NewEncoder(w).Encode(ibmJobResponse{ID: "job-retry"})
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, iamSrv, apiSrv)
	c, _ := builder.New("test", 1).H(0).Build()
	job, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if job.ID != "job-retry" {
		t.Errorf("job ID = %q, want %q", job.ID, "job-retry")
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("attempts = %d, want 3", got)
	}
}

func TestAuthError(t *testing.T) {
	iamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorCode":"BXNIM0415E","errorMessage":"Provided API key could not be found."}`))
	}))
	defer iamSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API should not be called when IAM auth fails")
	}))
	defer apiSrv.Close()

	b := newTestBackend(t, iamSrv, apiSrv)
	c, _ := builder.New("test", 1).H(0).Build()
	_, err := b.Submit(context.Background(), &backend.SubmitRequest{
		Circuit: c,
		Shots:   100,
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		s    string
		want backend.JobState
	}{
		{"Queued", backend.StateSubmitted},
		{"Running", backend.StateRunning},
		{"Completed", backend.StateCompleted},
		{"Failed", backend.StateFailed},
		{"Cancelled", backend.StateCancelled},
		{"Unknown", backend.StateSubmitted},
	}
	for _, tt := range tests {
		if got := parseState(tt.s); got != tt.want {
			t.Errorf("parseState(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}

func TestDeviceTarget(t *testing.T) {
	tests := []struct {
		device string
		want   string
	}{
		{"ibm_brisbane", "ibm.brisbane"},
		{"ibm_sherbrooke", "ibm.sherbrooke"},
		{"unknown_device", "ibm.brisbane"}, // default
	}
	for _, tt := range tests {
		got := deviceTarget(tt.device)
		if got.Name != tt.want {
			t.Errorf("deviceTarget(%q).Name = %q, want %q", tt.device, got.Name, tt.want)
		}
	}
}

func TestBackendName(t *testing.T) {
	b := New("key", "crn", WithDevice("ibm_sherbrooke"))
	if got := b.Name(); got != "ibm.ibm_sherbrooke" {
		t.Errorf("Name() = %q, want ibm.ibm_sherbrooke", got)
	}
}

func TestParseResults(t *testing.T) {
	resp := ibmResultResponse{
		Results: []ibmPubResult{{
			Data: ibmResultData{
				CRegSamples: map[string][][]int{
					"meas": {
						{0, 0, 0},
						{1, 0, 1},
						{1, 0, 1},
						{0, 1, 0},
					},
				},
			},
		}},
	}

	result, err := parseResults(resp, 3, 4)
	if err != nil {
		t.Fatal(err)
	}
	if result.Counts["000"] != 1 {
		t.Errorf("Counts[000] = %d, want 1", result.Counts["000"])
	}
	if result.Counts["101"] != 2 {
		t.Errorf("Counts[101] = %d, want 2", result.Counts["101"])
	}
	if result.Counts["010"] != 1 {
		t.Errorf("Counts[010] = %d, want 1", result.Counts["010"])
	}
	if result.Shots != 4 {
		t.Errorf("Shots = %d, want 4", result.Shots)
	}
}

func TestSampleToBitstring(t *testing.T) {
	tests := []struct {
		sample    []int
		numQubits int
		want      string
	}{
		{[]int{0, 0}, 2, "00"},
		{[]int{1, 1}, 2, "11"},
		{[]int{1, 0, 1}, 3, "101"},
		{[]int{1}, 3, "001"}, // padded
	}
	for _, tt := range tests {
		got := sampleToBitstring(tt.sample, tt.numQubits)
		if got != tt.want {
			t.Errorf("sampleToBitstring(%v, %d) = %q, want %q", tt.sample, tt.numQubits, got, tt.want)
		}
	}
}

func TestSerializeCircuit(t *testing.T) {
	c, _ := builder.New("test", 2).H(0).CNOT(0, 1).Build()
	qasm, err := serializeCircuit(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(qasm) == 0 {
		t.Error("expected non-empty QASM string")
	}
}
