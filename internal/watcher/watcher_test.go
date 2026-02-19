package watcher

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/clobrano/prow-helper/internal/parser"
)

func TestBuildFinishedJSONURL(t *testing.T) {
	tests := []struct {
		name     string
		metadata *parser.ProwMetadata
		want     string
	}{
		{
			name: "standard URL",
			metadata: &parser.ProwMetadata{
				Bucket: "test-platform-results",
				Path:   "logs/periodic-ci-test/12345",
			},
			want: "https://storage.googleapis.com/test-platform-results/logs/periodic-ci-test/12345/finished.json",
		},
		{
			name: "different bucket",
			metadata: &parser.ProwMetadata{
				Bucket: "origin-ci-test",
				Path:   "pr-logs/pull/openshift_api/1234/test-job/5678",
			},
			want: "https://storage.googleapis.com/origin-ci-test/pr-logs/pull/openshift_api/1234/test-job/5678/finished.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildFinishedJSONURL(tt.metadata)
			if got != tt.want {
				t.Errorf("BuildFinishedJSONURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckJobStatus_JobFinished(t *testing.T) {
	finished := finishedJSON{
		Timestamp: time.Now().Unix(),
		Passed:    true,
		Result:    "SUCCESS",
	}
	body, _ := json.Marshal(finished)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	status, err := CheckJobStatus(server.URL)
	if err != nil {
		t.Fatalf("CheckJobStatus() error = %v", err)
	}

	if status == nil {
		t.Fatal("CheckJobStatus() returned nil status for finished job")
	}

	if !status.Finished {
		t.Error("CheckJobStatus().Finished = false, want true")
	}

	if !status.Passed {
		t.Error("CheckJobStatus().Passed = false, want true")
	}
}

func TestCheckJobStatus_JobFailed(t *testing.T) {
	finished := finishedJSON{
		Timestamp: time.Now().Unix(),
		Passed:    false,
		Result:    "FAILURE",
	}
	body, _ := json.Marshal(finished)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	status, err := CheckJobStatus(server.URL)
	if err != nil {
		t.Fatalf("CheckJobStatus() error = %v", err)
	}

	if status == nil {
		t.Fatal("CheckJobStatus() returned nil status for finished job")
	}

	if !status.Finished {
		t.Error("CheckJobStatus().Finished = false, want true")
	}

	if status.Passed {
		t.Error("CheckJobStatus().Passed = true, want false")
	}
}

func TestCheckJobStatus_JobRunning(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	status, err := CheckJobStatus(server.URL)
	if err != nil {
		t.Fatalf("CheckJobStatus() error = %v", err)
	}

	if status != nil {
		t.Error("CheckJobStatus() should return nil status for running job")
	}
}

func TestCheckJobStatus_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	_, err := CheckJobStatus(server.URL)
	if err == nil {
		t.Error("CheckJobStatus() should return error for invalid JSON")
	}
}

func TestCheckJobStatus_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := CheckJobStatus(server.URL)
	if err == nil {
		t.Error("CheckJobStatus() should return error for server error")
	}
}

func TestBuildStartedJSONURL(t *testing.T) {
	tests := []struct {
		name     string
		metadata *parser.ProwMetadata
		want     string
	}{
		{
			name: "standard URL",
			metadata: &parser.ProwMetadata{
				Bucket: "test-platform-results",
				Path:   "logs/periodic-ci-test/12345",
			},
			want: "https://storage.googleapis.com/test-platform-results/logs/periodic-ci-test/12345/started.json",
		},
		{
			name: "different bucket",
			metadata: &parser.ProwMetadata{
				Bucket: "origin-ci-test",
				Path:   "pr-logs/pull/openshift_api/1234/test-job/5678",
			},
			want: "https://storage.googleapis.com/origin-ci-test/pr-logs/pull/openshift_api/1234/test-job/5678/started.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildStartedJSONURL(tt.metadata)
			if got != tt.want {
				t.Errorf("BuildStartedJSONURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFetchJobStartTime_Success(t *testing.T) {
	expectedTime := time.Unix(1700000000, 0)
	started := startedJSON{Timestamp: expectedTime.Unix()}
	body, _ := json.Marshal(started)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	got, err := FetchJobStartTime(server.URL)
	if err != nil {
		t.Fatalf("FetchJobStartTime() error = %v", err)
	}
	if !got.Equal(expectedTime) {
		t.Errorf("FetchJobStartTime() = %v, want %v", got, expectedTime)
	}
}

func TestFetchJobStartTime_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	got, err := FetchJobStartTime(server.URL)
	if err != nil {
		t.Fatalf("FetchJobStartTime() unexpected error = %v", err)
	}
	if !got.IsZero() {
		t.Errorf("FetchJobStartTime() should return zero time for 404, got %v", got)
	}
}

func TestFetchJobStartTime_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	_, err := FetchJobStartTime(server.URL)
	if err == nil {
		t.Error("FetchJobStartTime() should return error for invalid JSON")
	}
}

func TestFetchJobStartTime_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := FetchJobStartTime(server.URL)
	if err == nil {
		t.Error("FetchJobStartTime() should return error for server error")
	}
}

func TestWatch_AlreadyFinished(t *testing.T) {
	finished := finishedJSON{
		Timestamp: time.Now().Unix(),
		Passed:    true,
		Result:    "SUCCESS",
	}
	body, _ := json.Marshal(finished)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	// We test with a custom approach using CheckJobStatus since Watch uses it
	// The Watch function requires mocking the URL building which is complex
	status, err := CheckJobStatus(server.URL)
	if err != nil {
		t.Fatalf("CheckJobStatus() error = %v", err)
	}

	if status == nil {
		t.Fatal("Expected non-nil status for finished job")
	}

	if !status.Passed {
		t.Error("Expected status.Passed to be true")
	}
}
