package output

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

// Status represents job status for formatting
type Status int

const (
	StatusRunning Status = iota
	StatusSucceeded
	StatusFailed
)

var (
	// Bold is used for field labels
	Bold = color.New(color.Bold)

	// StatusColors maps status to colors
	greenBold  = color.New(color.FgGreen, color.Bold)
	redBold    = color.New(color.FgRed, color.Bold)
	yellowBold = color.New(color.FgYellow, color.Bold)
)

// StatusInfo contains display information for a status
type StatusInfo struct {
	Emoji string
	Text  string
	Color *color.Color
}

// GetStatusInfo returns the display information for a status
func GetStatusInfo(status Status) StatusInfo {
	switch status {
	case StatusSucceeded:
		return StatusInfo{Emoji: "✅", Text: "PASSED", Color: greenBold}
	case StatusFailed:
		return StatusInfo{Emoji: "❌", Text: "FAILED", Color: redBold}
	case StatusRunning:
		return StatusInfo{Emoji: "🔄", Text: "RUNNING", Color: yellowBold}
	default:
		return StatusInfo{Emoji: "", Text: "UNKNOWN", Color: Bold}
	}
}

// PrintField prints a field with bold label
func PrintField(w io.Writer, label, value string) {
	Bold.Fprintf(w, "%s: ", label)
	fmt.Fprintln(w, value)
}

// PrintStatus prints a status with emoji and color
func PrintStatus(w io.Writer, status Status) {
	info := GetStatusInfo(status)
	info.Color.Fprintf(w, "%s %s\n", info.Emoji, info.Text)
}

// FormatStatus returns a formatted status string with emoji and color
func FormatStatus(status Status) string {
	info := GetStatusInfo(status)
	return info.Color.Sprintf("%s %s", info.Emoji, info.Text)
}

// FormatJobStatusMessage creates a message for job completion status
func FormatJobStatusMessage(jobName string, passed bool) string {
	status := StatusSucceeded
	if !passed {
		status = StatusFailed
	}
	info := GetStatusInfo(status)
	return fmt.Sprintf("Job %s completed: %s", jobName, info.Color.Sprintf("%s %s", info.Emoji, info.Text))
}
