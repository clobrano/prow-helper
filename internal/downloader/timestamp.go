package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StartedMetadata represents the structure of the started.json file
type StartedMetadata struct {
	Timestamp int64 `json:"timestamp"`
}

// ReadStartedTimestamp reads the started.json file and extracts the timestamp
func ReadStartedTimestamp(artifactPath string) (time.Time, error) {
	startedFilePath := filepath.Join(artifactPath, "started.json")

	data, err := os.ReadFile(startedFilePath)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to read started.json: %w", err)
	}

	var metadata StartedMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return time.Time{}, fmt.Errorf("failed to parse started.json: %w", err)
	}

	if metadata.Timestamp == 0 {
		return time.Time{}, fmt.Errorf("timestamp field is missing or zero in started.json")
	}

	// Convert Unix timestamp to time.Time
	timestamp := time.Unix(metadata.Timestamp, 0)
	return timestamp, nil
}

// FormatTimestampPrefix formats a timestamp as YYYYMMDD-HHMM for use as a folder prefix
func FormatTimestampPrefix(t time.Time) string {
	return t.Format("20060102-1504")
}

// RenameWithDatePrefix renames a folder to include a date prefix from started.json
// Returns the new path after renaming
func RenameWithDatePrefix(artifactPath string) (string, error) {
	// Read timestamp from started.json
	timestamp, err := ReadStartedTimestamp(artifactPath)
	if err != nil {
		return "", err
	}

	// Format the timestamp prefix
	prefix := FormatTimestampPrefix(timestamp)

	// Get the parent directory and current folder name
	parentDir := filepath.Dir(artifactPath)
	currentName := filepath.Base(artifactPath)

	// Create new path with date prefix
	newName := prefix + "-" + currentName
	newPath := filepath.Join(parentDir, newName)

	// Rename the folder
	if err := os.Rename(artifactPath, newPath); err != nil {
		return "", fmt.Errorf("failed to rename folder: %w", err)
	}

	return newPath, nil
}
