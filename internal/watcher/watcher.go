package watcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/clobrano/prow-helper/internal/parser"
)

const (
	// DefaultPollInterval is the default time between status checks
	DefaultPollInterval = 15 * time.Minute

	// GCSBaseURL is the base URL for Google Cloud Storage
	GCSBaseURL = "https://storage.googleapis.com"
)

// JobStatus represents the current status of a Prow job
type JobStatus struct {
	Finished  bool
	Passed    bool
	Timestamp time.Time
}

// finishedJSON represents the structure of finished.json from Prow
type finishedJSON struct {
	Timestamp int64  `json:"timestamp"`
	Passed    bool   `json:"passed"`
	Result    string `json:"result"`
}

// BuildFinishedJSONURL converts a Prow URL to the GCS finished.json URL.
// Prow URL: https://prow.ci.openshift.org/view/gs/<bucket>/<path>
// GCS URL:  https://storage.googleapis.com/<bucket>/<path>/finished.json
func BuildFinishedJSONURL(metadata *parser.ProwMetadata) string {
	return fmt.Sprintf("%s/%s/%s/finished.json", GCSBaseURL, metadata.Bucket, metadata.Path)
}

// CheckJobStatus fetches finished.json and returns the job status.
// Returns nil status if the job is still running (404 response).
func CheckJobStatus(finishedURL string) (*JobStatus, error) {
	resp, err := http.Get(finishedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch job status: %w", err)
	}
	defer resp.Body.Close()

	// 404 means job is still running
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var finished finishedJSON
	if err := json.Unmarshal(body, &finished); err != nil {
		return nil, fmt.Errorf("failed to parse finished.json: %w", err)
	}

	return &JobStatus{
		Finished:  true,
		Passed:    finished.Passed,
		Timestamp: time.Unix(finished.Timestamp, 0),
	}, nil
}

// Watch polls the job status until the job completes.
// It checks finished.json at the specified interval until the job finishes.
// Returns the final job status when complete.
func Watch(metadata *parser.ProwMetadata, interval time.Duration, output io.Writer) (*JobStatus, error) {
	finishedURL := BuildFinishedJSONURL(metadata)

	fmt.Fprintf(output, "Watching job: %s\n", metadata.JobName)
	fmt.Fprintf(output, "Build ID: %s\n", metadata.BuildID)
	fmt.Fprintf(output, "Polling interval: %s\n", interval)
	fmt.Fprintf(output, "Checking: %s\n", finishedURL)

	// Check immediately first
	status, err := CheckJobStatus(finishedURL)
	if err != nil {
		return nil, err
	}
	if status != nil {
		fmt.Fprintf(output, "Job already finished\n")
		return status, nil
	}

	fmt.Fprintf(output, "Job is running, waiting for completion...\n")

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		fmt.Fprintf(output, "[%s] Checking job status...\n", time.Now().Format(time.RFC3339))

		status, err := CheckJobStatus(finishedURL)
		if err != nil {
			fmt.Fprintf(output, "Warning: %v\n", err)
			continue
		}

		if status != nil {
			fmt.Fprintf(output, "Job completed!\n")
			return status, nil
		}
	}

	// This should never be reached
	return nil, fmt.Errorf("watch loop unexpectedly terminated")
}
