package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/clobrano/prow-helper/internal/config"
	"github.com/clobrano/prow-helper/internal/parser"
)

func TestURLValidationIntegration(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid PROW URL",
			url:     "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296",
			wantErr: false,
		},
		{
			name:    "invalid URL - wrong host",
			url:     "https://example.com/view/gs/bucket/path/123",
			wantErr: true,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigLoadingIntegration(t *testing.T) {
	// Create a temporary XDG config directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "prow-helper")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Write a test config file
	configPath := filepath.Join(configDir, "config.yaml")
	configContent := `dest: /tmp/test-artifacts
analyze_cmd: "echo test"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Test loading the config file directly
	cfg, err := config.LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}

	if cfg.Dest != "/tmp/test-artifacts" {
		t.Errorf("Config.Dest = %v, want /tmp/test-artifacts", cfg.Dest)
	}
	if cfg.AnalyzeCmd != "echo test" {
		t.Errorf("Config.AnalyzeCmd = %v, want 'echo test'", cfg.AnalyzeCmd)
	}
}

func TestConfigMergingIntegration(t *testing.T) {
	// Test that CLI overrides everything
	cliConfig := &config.Config{Dest: "/cli/path", AnalyzeCmd: "cli-cmd"}
	envConfig := &config.Config{Dest: "/env/path", AnalyzeCmd: "env-cmd"}
	fileConfig := &config.Config{Dest: "/file/path", AnalyzeCmd: "file-cmd"}
	defaults := config.DefaultConfig()

	merged := config.MergeConfig(cliConfig, envConfig, fileConfig, defaults)

	if merged.Dest != "/cli/path" {
		t.Errorf("Merged.Dest = %v, want /cli/path (CLI should override)", merged.Dest)
	}
	if merged.AnalyzeCmd != "cli-cmd" {
		t.Errorf("Merged.AnalyzeCmd = %v, want cli-cmd (CLI should override)", merged.AnalyzeCmd)
	}
}

func TestURLParsingIntegration(t *testing.T) {
	url := "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296"

	metadata, err := parser.ParseURL(url)
	if err != nil {
		t.Fatalf("ParseURL() error = %v", err)
	}

	if metadata.Bucket != "test-platform-results" {
		t.Errorf("Metadata.Bucket = %v, want test-platform-results", metadata.Bucket)
	}
	if metadata.JobName != "periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview" {
		t.Errorf("Metadata.JobName = %v, unexpected", metadata.JobName)
	}
	if metadata.BuildID != "2013057817195319296" {
		t.Errorf("Metadata.BuildID = %v, want 2013057817195319296", metadata.BuildID)
	}
}

func TestGsutilCommandConstruction(t *testing.T) {
	metadata := &parser.ProwMetadata{
		Bucket:  "test-platform-results",
		Path:    "logs/test-job/12345",
		JobName: "test-job",
		BuildID: "12345",
	}

	cmd := parser.BuildGsutilCommand(metadata, "/tmp/dest")
	expected := "gsutil -m cp -r gs://test-platform-results/logs/test-job/12345/ /tmp/dest"

	if cmd != expected {
		t.Errorf("BuildGsutilCommand() = %v, want %v", cmd, expected)
	}
}

// TestExitCodes verifies that the application uses the correct exit codes
// This is a documentation test - actual exit code testing requires running the binary
func TestExitCodeConstants(t *testing.T) {
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess = %d, want 0", ExitSuccess)
	}
	if ExitInvalidURL != 1 {
		t.Errorf("ExitInvalidURL = %d, want 1", ExitInvalidURL)
	}
	if ExitDownloadFailed != 2 {
		t.Errorf("ExitDownloadFailed = %d, want 2", ExitDownloadFailed)
	}
	if ExitAnalysisFailed != 3 {
		t.Errorf("ExitAnalysisFailed = %d, want 3", ExitAnalysisFailed)
	}
}
