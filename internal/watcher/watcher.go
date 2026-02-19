package watcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/clobrano/prow-helper/internal/output"
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

// startedJSON represents the structure of started.json from Prow
type startedJSON struct {
	Timestamp int64 `json:"timestamp"`
}

// BuildFinishedJSONURL converts a Prow URL to the GCS finished.json URL.
// Prow URL: https://prow.ci.openshift.org/view/gs/<bucket>/<path>
// GCS URL:  https://storage.googleapis.com/<bucket>/<path>/finished.json
func BuildFinishedJSONURL(metadata *parser.ProwMetadata) string {
	return fmt.Sprintf("%s/%s/%s/finished.json", GCSBaseURL, metadata.Bucket, metadata.Path)
}

// BuildStartedJSONURL converts a Prow URL to the GCS started.json URL.
// GCS URL: https://storage.googleapis.com/<bucket>/<path>/started.json
func BuildStartedJSONURL(metadata *parser.ProwMetadata) string {
	return fmt.Sprintf("%s/%s/%s/started.json", GCSBaseURL, metadata.Bucket, metadata.Path)
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

// FetchJobStartTime fetches started.json and returns the job start time.
// Returns a zero time.Time if the file is not yet available (404).
func FetchJobStartTime(startedURL string) (time.Time, error) {
	resp, err := http.Get(startedURL)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to fetch started.json: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return time.Time{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("unexpected status code fetching started.json: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read started.json body: %w", err)
	}

	var started startedJSON
	if err := json.Unmarshal(body, &started); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse started.json: %w", err)
	}

	if started.Timestamp == 0 {
		return time.Time{}, nil
	}

	return time.Unix(started.Timestamp, 0), nil
}

// Watch polls the job status until the job completes.
// It checks finished.json at the specified interval until the job finishes.
// Returns the final job status when complete.
func Watch(metadata *parser.ProwMetadata, interval time.Duration, w io.Writer) (*JobStatus, error) {
	finishedURL := BuildFinishedJSONURL(metadata)

	output.PrintField(w, "Watching job", metadata.JobName)
	output.PrintField(w, "Build ID", metadata.BuildID)
	if metadata.RawURL != "" {
		output.PrintField(w, "Job page", metadata.RawURL)
	}
	output.PrintField(w, "Polling interval", interval.String())
	output.PrintField(w, "Checking", finishedURL)

	// Fetch job start time from started.json (best-effort)
	startedURL := BuildStartedJSONURL(metadata)
	startTime, err := FetchJobStartTime(startedURL)
	if err != nil {
		fmt.Fprintf(w, "Note: could not fetch job start time: %v\n", err)
	}
	if !startTime.IsZero() {
		output.PrintField(w, "Started at", startTime.Format("2006-01-02 15:04:05"))
	}

	// Check immediately first
	status, err := CheckJobStatus(finishedURL)
	if err != nil {
		return nil, err
	}
	if status != nil {
		fmt.Fprintf(w, "Job already finished\n")
		return status, nil
	}

	fmt.Fprintf(w, "Job is running, waiting for completion...\n")
	output.PrintStatus(w, output.StatusRunning)

	checkTicker := time.NewTicker(interval)
	defer checkTicker.Stop()
	countdownTicker := time.NewTicker(time.Second)
	defer countdownTicker.Stop()

	lastCheckTime := time.Now()
	nextCheckTime := lastCheckTime.Add(interval)
	printCountdown(w, startTime, lastCheckTime, nextCheckTime)

	for {
		select {
		case t := <-checkTicker.C:
			status, err := CheckJobStatus(finishedURL)
			if err != nil {
				fmt.Fprintf(w, "\r%-100s\n", fmt.Sprintf("Warning: %v", err))
			} else if status != nil {
				fmt.Fprintf(w, "\r%-100s\n", "Job completed!")
				return status, nil
			}
			lastCheckTime = t
			nextCheckTime = t.Add(interval)
			printCountdown(w, startTime, lastCheckTime, nextCheckTime)

		case <-countdownTicker.C:
			printCountdown(w, startTime, lastCheckTime, nextCheckTime)
		}
	}
}

// printCountdown overwrites the current terminal line with elapsed time since
// the job started, the last check time, and a live countdown to the next check.
func printCountdown(w io.Writer, startTime, lastCheck, nextCheck time.Time) {
	timeLeft := time.Until(nextCheck).Truncate(time.Second)
	if timeLeft < 0 {
		timeLeft = 0
	}

	var parts []string
	if !startTime.IsZero() {
		elapsed := time.Since(startTime).Truncate(time.Second)
		parts = append(parts, fmt.Sprintf("[started: %s]", startTime.Format("15:04:05")))
		parts = append(parts, fmt.Sprintf("[running for: %s]", elapsed))
	}
	parts = append(parts, fmt.Sprintf("[last check: %s]", lastCheck.Format("15:04:05")))
	parts = append(parts, fmt.Sprintf("[next check in: %s]", timeLeft))

	fmt.Fprintf(w, "\r%-100s", strings.Join(parts, " "))
}
