package notifier

import (
	"fmt"
	"os/exec"

	"github.com/gen2brain/beeep"
)

// Notify sends a desktop notification with the given title and message.
// Uses beeep library which supports Linux (notify-send), macOS, and Windows.
// If beeep fails, falls back to notify-send command directly.
func Notify(title, message string, success bool) error {
	// Add status indicator to title
	statusIcon := "Success"
	if !success {
		statusIcon = "Failed"
	}
	fullTitle := fmt.Sprintf("prow-helper: %s - %s", title, statusIcon)

	// Try beeep first
	err := beeep.Notify(fullTitle, message, "")
	if err == nil {
		return nil
	}

	// Fallback to notify-send for Linux
	return notifySendFallback(fullTitle, message)
}

// notifySendFallback uses the notify-send command directly.
func notifySendFallback(title, message string) error {
	cmd := exec.Command("notify-send", title, message)
	return cmd.Run()
}

// FormatSuccessMessage creates a success notification message.
func FormatSuccessMessage(jobName, destPath string) string {
	return fmt.Sprintf("Artifacts downloaded to:\n%s\n\nJob: %s", destPath, jobName)
}

// FormatFailureMessage creates a failure notification message.
func FormatFailureMessage(jobName string, err error) string {
	return fmt.Sprintf("Job: %s\n\nError: %s", jobName, err.Error())
}

// FormatAnalysisSuccessMessage creates a success message for completed analysis.
func FormatAnalysisSuccessMessage(jobName, destPath string) string {
	return fmt.Sprintf("Analysis complete for:\n%s\n\nArtifacts: %s", jobName, destPath)
}

// FormatDownloadOnlyMessage creates a message when only download was performed.
func FormatDownloadOnlyMessage(jobName, destPath string) string {
	return fmt.Sprintf("Download complete for:\n%s\n\nArtifacts: %s", jobName, destPath)
}
