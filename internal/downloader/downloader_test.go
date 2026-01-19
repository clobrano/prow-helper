package downloader

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clobrano/prow-helper/internal/parser"
)

func TestBuildDestinationPath(t *testing.T) {
	tests := []struct {
		name     string
		baseDest string
		metadata *parser.ProwMetadata
		wantEnd  string // Check if path ends with this
	}{
		{
			name:     "absolute path",
			baseDest: "/tmp/artifacts",
			metadata: &parser.ProwMetadata{
				JobName: "test-job",
				BuildID: "12345",
			},
			wantEnd: "/tmp/artifacts/test-job/12345",
		},
		{
			name:     "relative path",
			baseDest: ".",
			metadata: &parser.ProwMetadata{
				JobName: "my-job",
				BuildID: "67890",
			},
			wantEnd: "my-job/67890",
		},
		{
			name:     "complex job name",
			baseDest: "/data",
			metadata: &parser.ProwMetadata{
				JobName: "periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn",
				BuildID: "2013057817195319296",
			},
			wantEnd: "/data/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn/2013057817195319296",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDestinationPath(tt.baseDest, tt.metadata)
			if !strings.HasSuffix(got, tt.wantEnd) {
				t.Errorf("BuildDestinationPath() = %v, want suffix %v", got, tt.wantEnd)
			}
		})
	}
}

func TestBuildDestinationPath_HomeExpansion(t *testing.T) {
	metadata := &parser.ProwMetadata{
		JobName: "test-job",
		BuildID: "123",
	}

	got := BuildDestinationPath("~/prow-artifacts", metadata)

	// Should not start with ~/
	if strings.HasPrefix(got, "~/") {
		t.Errorf("BuildDestinationPath() = %v, should expand ~ to home directory", got)
	}

	// Should contain the job and build
	if !strings.Contains(got, "test-job") || !strings.Contains(got, "123") {
		t.Errorf("BuildDestinationPath() = %v, should contain job name and build ID", got)
	}
}

func TestCheckDestinationConflict(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Test existing directory
	exists, err := CheckDestinationConflict(tmpDir)
	if err != nil {
		t.Fatalf("CheckDestinationConflict() error = %v", err)
	}
	if !exists {
		t.Error("CheckDestinationConflict() = false, want true for existing directory")
	}

	// Test non-existing path
	nonExistent := filepath.Join(tmpDir, "nonexistent")
	exists, err = CheckDestinationConflict(nonExistent)
	if err != nil {
		t.Fatalf("CheckDestinationConflict() error = %v", err)
	}
	if exists {
		t.Error("CheckDestinationConflict() = true, want false for non-existing path")
	}
}

func TestCreateTimestampedPath(t *testing.T) {
	basePath := "/tmp/artifacts/test-job/123"
	got := CreateTimestampedPath(basePath)

	if !strings.HasPrefix(got, basePath+"-") {
		t.Errorf("CreateTimestampedPath() = %v, should start with %v-", got, basePath)
	}

	// Should have format like -20060102-150405
	if len(got) != len(basePath)+16 { // +1 for dash, +15 for timestamp
		t.Errorf("CreateTimestampedPath() = %v, unexpected length", got)
	}
}

func TestPromptConflictResolution(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ConflictResolution
	}{
		{"overwrite lowercase", "o\n", Overwrite},
		{"overwrite full", "overwrite\n", Overwrite},
		{"skip lowercase", "s\n", Skip},
		{"skip full", "skip\n", Skip},
		{"new lowercase", "n\n", NewTimestamped},
		{"new full", "new\n", NewTimestamped},
		{"empty defaults to overwrite", "\n", Overwrite},
		{"unknown defaults to overwrite", "x\n", Overwrite},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdin := strings.NewReader(tt.input)
			stdout := &bytes.Buffer{}

			got, err := PromptConflictResolution("/some/path", stdin, stdout)
			if err != nil {
				t.Fatalf("PromptConflictResolution() error = %v", err)
			}
			if got != tt.expected {
				t.Errorf("PromptConflictResolution() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCheckGsutilAvailable(t *testing.T) {
	// This test will pass if gsutil is installed, fail if not
	// We just check that it returns either nil or ErrGsutilNotFound
	err := CheckGsutilAvailable()
	if err != nil && err != ErrGsutilNotFound {
		t.Errorf("CheckGsutilAvailable() unexpected error = %v", err)
	}
}

func TestResolveDestination_NoConflict(t *testing.T) {
	tmpDir := t.TempDir()
	metadata := &parser.ProwMetadata{
		JobName: "new-job",
		BuildID: "999",
	}

	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}

	destPath, skip, err := ResolveDestination(tmpDir, metadata, stdin, stdout)
	if err != nil {
		t.Fatalf("ResolveDestination() error = %v", err)
	}
	if skip {
		t.Error("ResolveDestination() skip = true, want false for new destination")
	}
	if !strings.Contains(destPath, "new-job") || !strings.Contains(destPath, "999") {
		t.Errorf("ResolveDestination() = %v, should contain job name and build ID", destPath)
	}
}

func TestResolveDestination_Skip(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the directory that would conflict
	metadata := &parser.ProwMetadata{
		JobName: "existing-job",
		BuildID: "111",
	}
	existingPath := filepath.Join(tmpDir, "existing-job", "111")
	if err := os.MkdirAll(existingPath, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	stdin := strings.NewReader("s\n")
	stdout := &bytes.Buffer{}

	_, skip, err := ResolveDestination(tmpDir, metadata, stdin, stdout)
	if err != nil {
		t.Fatalf("ResolveDestination() error = %v", err)
	}
	if !skip {
		t.Error("ResolveDestination() skip = false, want true when user chooses skip")
	}
}

func TestResolveDestination_NewTimestamped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the directory that would conflict
	metadata := &parser.ProwMetadata{
		JobName: "existing-job",
		BuildID: "222",
	}
	existingPath := filepath.Join(tmpDir, "existing-job", "222")
	if err := os.MkdirAll(existingPath, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	stdin := strings.NewReader("n\n")
	stdout := &bytes.Buffer{}

	destPath, skip, err := ResolveDestination(tmpDir, metadata, stdin, stdout)
	if err != nil {
		t.Fatalf("ResolveDestination() error = %v", err)
	}
	if skip {
		t.Error("ResolveDestination() skip = true, want false for new timestamped")
	}
	if !strings.Contains(destPath, "existing-job") || !strings.Contains(destPath, "222-") {
		t.Errorf("ResolveDestination() = %v, should be timestamped version", destPath)
	}
}
