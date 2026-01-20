package downloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadStartedTimestamp(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		expectedTS  int64
	}{
		{
			name:       "valid started.json",
			content:    `{"timestamp": 1595278460}`,
			wantErr:    false,
			expectedTS: 1595278460,
		},
		{
			name:       "valid started.json with other fields",
			content:    `{"timestamp": 1595277241, "node": "test-node", "pull": "12345"}`,
			wantErr:    false,
			expectedTS: 1595277241,
		},
		{
			name:    "missing timestamp field",
			content: `{"node": "test-node"}`,
			wantErr: true,
		},
		{
			name:    "invalid json",
			content: `{invalid json}`,
			wantErr: true,
		},
		{
			name:    "zero timestamp",
			content: `{"timestamp": 0}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()
			startedFile := filepath.Join(tmpDir, "started.json")

			// Write test content
			if err := os.WriteFile(startedFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Test ReadStartedTimestamp
			timestamp, err := ReadStartedTimestamp(tmpDir)

			if (err != nil) != tt.wantErr {
				t.Errorf("ReadStartedTimestamp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if timestamp.Unix() != tt.expectedTS {
					t.Errorf("ReadStartedTimestamp() = %v, want %v", timestamp.Unix(), tt.expectedTS)
				}
			}
		})
	}
}

func TestReadStartedTimestamp_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := ReadStartedTimestamp(tmpDir)
	if err == nil {
		t.Error("Expected error when started.json is missing, got nil")
	}
}

func TestFormatTimestampPrefix(t *testing.T) {
	tests := []struct {
		name      string
		timestamp time.Time
		want      string
	}{
		{
			name:      "timestamp from example 1",
			timestamp: time.Unix(1595278460, 0).UTC(),
			want:      "20200720-2101",
		},
		{
			name:      "timestamp from example 2",
			timestamp: time.Unix(1595277241, 0).UTC(),
			want:      "20200720-2040",
		},
		{
			name:      "new year timestamp",
			timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			want:      "20240101-0000",
		},
		{
			name:      "timestamp with double digit hour and minute",
			timestamp: time.Date(2024, 12, 31, 23, 59, 0, 0, time.UTC),
			want:      "20241231-2359",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTimestampPrefix(tt.timestamp)
			if got != tt.want {
				t.Errorf("FormatTimestampPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenameWithDatePrefix(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "artifacts")
	artifactDir := filepath.Join(parentDir, "job-12345")

	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create started.json with known timestamp
	startedContent := `{"timestamp": 1595278460}`
	startedFile := filepath.Join(artifactDir, "started.json")
	if err := os.WriteFile(startedFile, []byte(startedContent), 0644); err != nil {
		t.Fatalf("Failed to create started.json: %v", err)
	}

	// Test RenameWithDatePrefix
	newPath, err := RenameWithDatePrefix(artifactDir)
	if err != nil {
		t.Fatalf("RenameWithDatePrefix() error = %v", err)
	}

	// Verify new path format
	expectedName := "20200720-2101-job-12345"
	expectedPath := filepath.Join(parentDir, expectedName)

	if newPath != expectedPath {
		t.Errorf("RenameWithDatePrefix() = %v, want %v", newPath, expectedPath)
	}

	// Verify directory was actually renamed
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		t.Error("Renamed directory does not exist")
	}

	// Verify old directory no longer exists
	if _, err := os.Stat(artifactDir); !os.IsNotExist(err) {
		t.Error("Old directory still exists after rename")
	}

	// Verify started.json exists in new location
	newStartedFile := filepath.Join(newPath, "started.json")
	if _, err := os.Stat(newStartedFile); os.IsNotExist(err) {
		t.Error("started.json does not exist in renamed directory")
	}
}

func TestRenameWithDatePrefix_MissingStartedJSON(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := RenameWithDatePrefix(tmpDir)
	if err == nil {
		t.Error("Expected error when started.json is missing, got nil")
	}
}
