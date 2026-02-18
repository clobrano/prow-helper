package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestGetStatusInfo(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		emoji  string
		text   string
	}{
		{
			name:   "succeeded status",
			status: StatusSucceeded,
			emoji:  "✅",
			text:   "PASSED",
		},
		{
			name:   "failed status",
			status: StatusFailed,
			emoji:  "❌",
			text:   "FAILED",
		},
		{
			name:   "running status",
			status: StatusRunning,
			emoji:  "🔄",
			text:   "RUNNING",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := GetStatusInfo(tt.status)
			if info.Emoji != tt.emoji {
				t.Errorf("GetStatusInfo().Emoji = %s, want %s", info.Emoji, tt.emoji)
			}
			if info.Text != tt.text {
				t.Errorf("GetStatusInfo().Text = %s, want %s", info.Text, tt.text)
			}
			if info.Color == nil {
				t.Error("GetStatusInfo().Color should not be nil")
			}
		})
	}
}

func TestPrintField(t *testing.T) {
	var buf bytes.Buffer
	PrintField(&buf, "Job", "test-job")

	output := buf.String()
	if !strings.Contains(output, "Job") {
		t.Error("PrintField output should contain label")
	}
	if !strings.Contains(output, "test-job") {
		t.Error("PrintField output should contain value")
	}
}

func TestPrintStatus(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   string
	}{
		{
			name:   "succeeded",
			status: StatusSucceeded,
			want:   "PASSED",
		},
		{
			name:   "failed",
			status: StatusFailed,
			want:   "FAILED",
		},
		{
			name:   "running",
			status: StatusRunning,
			want:   "RUNNING",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			PrintStatus(&buf, tt.status)

			output := buf.String()
			if !strings.Contains(output, tt.want) {
				t.Errorf("PrintStatus() output should contain %s, got %s", tt.want, output)
			}
		})
	}
}

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
