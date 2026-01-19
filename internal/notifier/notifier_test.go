package notifier

import (
	"errors"
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

// Note: We don't test Notify() directly as it interacts with system notifications
// Integration tests should verify notification delivery manually
