package notifier

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFormatSuccessMessage(t *testing.T) {
	msg := FormatSuccessMessage("test-job", "/path/to/artifacts")

	if !strings.Contains(msg, "test-job") {
		t.Error("FormatSuccessMessage should contain job name")
	}
	if !strings.Contains(msg, "/path/to/artifacts") {
		t.Error("FormatSuccessMessage should contain destination path")
	}
}

func TestFormatFailureMessage(t *testing.T) {
	err := errors.New("download failed")
	msg := FormatFailureMessage("test-job", err)

	if !strings.Contains(msg, "test-job") {
		t.Error("FormatFailureMessage should contain job name")
	}
	if !strings.Contains(msg, "download failed") {
		t.Error("FormatFailureMessage should contain error message")
	}
}

func TestFormatAnalysisSuccessMessage(t *testing.T) {
	msg := FormatAnalysisSuccessMessage("test-job", "/path/to/artifacts")

	if !strings.Contains(msg, "test-job") {
		t.Error("FormatAnalysisSuccessMessage should contain job name")
	}
	if !strings.Contains(msg, "/path/to/artifacts") {
		t.Error("FormatAnalysisSuccessMessage should contain destination path")
	}
	if !strings.Contains(msg, "Analysis") {
		t.Error("FormatAnalysisSuccessMessage should mention analysis")
	}
}

func TestFormatDownloadOnlyMessage(t *testing.T) {
	msg := FormatDownloadOnlyMessage("test-job", "/path/to/artifacts")

	if !strings.Contains(msg, "test-job") {
		t.Error("FormatDownloadOnlyMessage should contain job name")
	}
	if !strings.Contains(msg, "/path/to/artifacts") {
		t.Error("FormatDownloadOnlyMessage should contain destination path")
	}
	if !strings.Contains(msg, "Download") {
		t.Error("FormatDownloadOnlyMessage should mention download")
	}
}

func TestFormatDownloadStartMessage(t *testing.T) {
	msg := FormatDownloadStartMessage("test-job")

	if !strings.Contains(msg, "test-job") {
		t.Error("FormatDownloadStartMessage should contain job name")
	}
	if !strings.Contains(msg, "Starting download") {
		t.Error("FormatDownloadStartMessage should mention starting download")
	}
}

func TestFormatDownloadCompleteMessage(t *testing.T) {
	msg := FormatDownloadCompleteMessage("test-job", "/path/to/artifacts")

	if !strings.Contains(msg, "test-job") {
		t.Error("FormatDownloadCompleteMessage should contain job name")
	}
	if !strings.Contains(msg, "Download complete") {
		t.Error("FormatDownloadCompleteMessage should mention download complete")
	}
	if !strings.Contains(msg, "Starting analysis") {
		t.Error("FormatDownloadCompleteMessage should mention starting analysis")
	}
}

func TestFormatAnalysisStartMessage(t *testing.T) {
	msg := FormatAnalysisStartMessage("test-job", "my-analyzer")

	if !strings.Contains(msg, "test-job") {
		t.Error("FormatAnalysisStartMessage should contain job name")
	}
	if !strings.Contains(msg, "my-analyzer") {
		t.Error("FormatAnalysisStartMessage should contain analyze command")
	}
	if !strings.Contains(msg, "Starting analysis") {
		t.Error("FormatAnalysisStartMessage should mention starting analysis")
	}
}

// Note: We don't test Notify() directly as it interacts with system notifications
// Integration tests should verify notification delivery manually

func TestFormatJobStatusMessage(t *testing.T) {
	tests := []struct {
		name    string
		jobName string
		passed  bool
		want    string
	}{
		{
			name:    "passed job",
			jobName: "test-job",
			passed:  true,
			want:    "PASSED",
		},
		{
			name:    "failed job",
			jobName: "test-job",
			passed:  false,
			want:    "FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := FormatJobStatusMessage(tt.jobName, tt.passed)
			if !strings.Contains(msg, tt.jobName) {
				t.Errorf("FormatJobStatusMessage() should contain job name %s", tt.jobName)
			}
			if !strings.Contains(msg, tt.want) {
				t.Errorf("FormatJobStatusMessage() should contain status %s", tt.want)
			}
		})
	}
}

func TestNotifyNtfy(t *testing.T) {
	// Test with mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify title header
		title := r.Header.Get("Title")
		if title == "" {
			t.Error("Expected Title header to be set")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// We can't easily test NotifyNtfy directly because it uses hardcoded URL
	// This test documents the expected behavior
}
