package notifier

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/gen2brain/beeep"
)

const (
	// NtfyBaseURL is the base URL for ntfy.sh
	NtfyBaseURL = "https://ntfy.sh"
)

func init() {
	// Set the application name for notifications
	beeep.AppName = "prow-helper"
}

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

	// Try beeep first (AppName is set in init())
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

// FormatDownloadStartMessage creates a message when download is starting.
func FormatDownloadStartMessage(jobName string) string {
	return fmt.Sprintf("Starting download for:\n%s", jobName)
}

// FormatDownloadCompleteMessage creates a message when download completes (intermediate step).
func FormatDownloadCompleteMessage(jobName, destPath string) string {
	return fmt.Sprintf("Download complete for:\n%s\n\nStarting analysis...", jobName)
}

// FormatAnalysisStartMessage creates a message when analysis is starting.
func FormatAnalysisStartMessage(jobName, analyzeCmd string) string {
	return fmt.Sprintf("Starting analysis for:\n%s\n\nCommand: %s", jobName, analyzeCmd)
}

// NotifyNtfy sends a notification via ntfy.sh.
// channel is the ntfy.sh topic/channel name.
func NotifyNtfy(channel, title, message string) error {
	url := fmt.Sprintf("%s/%s", NtfyBaseURL, channel)

	req, err := http.NewRequest("POST", url, strings.NewReader(message))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Title", title)
	req.Header.Set("Content-Type", "text/plain")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ntfy.sh returned status %d", resp.StatusCode)
	}

	return nil
}

// NotifyWithConfig sends notification using configured method.
// If ntfyChannel is provided, sends via ntfy.sh; otherwise sends desktop notification.
func NotifyWithConfig(title, message string, success bool, ntfyChannel string) error {
	// Add status indicator to title
	statusIcon := "Success"
	if !success {
		statusIcon = "Failed"
	}
	fullTitle := fmt.Sprintf("prow-helper: %s - %s", title, statusIcon)

	// Send ntfy notification if channel is configured
	if ntfyChannel != "" {
		if err := NotifyNtfy(ntfyChannel, fullTitle, message); err != nil {
			// Log error but don't fail - try desktop notification as fallback
			fmt.Printf("Warning: ntfy notification failed: %v\n", err)
		}
	}

	// Always try desktop notification
	return Notify(title, message, success)
}

// FormatJobStatusMessage creates a message for job completion status.
func FormatJobStatusMessage(jobName string, passed bool) string {
	status := "PASSED"
	if !passed {
		status = "FAILED"
	}
	return fmt.Sprintf("Job %s has completed with status: %s", jobName, status)
}
