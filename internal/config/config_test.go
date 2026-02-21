package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Dest != "." {
		t.Errorf("DefaultConfig().Dest = %v, want %v", cfg.Dest, ".")
	}
	if cfg.AnalyzeCmd != "" {
		t.Errorf("DefaultConfig().AnalyzeCmd = %v, want empty string", cfg.AnalyzeCmd)
	}
	if cfg.NtfyChannel != "" {
		t.Errorf("DefaultConfig().NtfyChannel = %v, want empty string", cfg.NtfyChannel)
	}
	if cfg.Interactive {
		t.Error("DefaultConfig().Interactive = true, want false")
	}
}

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath()

	// Should contain prow-helper/config.yaml
	if !strings.Contains(path, "prow-helper") {
		t.Errorf("GetConfigPath() = %v, should contain 'prow-helper'", path)
	}
	if !strings.HasSuffix(path, "config.yaml") {
		t.Errorf("GetConfigPath() = %v, should end with 'config.yaml'", path)
	}

	// Should be under XDG config home (either custom or default ~/.config)
	if !strings.Contains(path, ".config") && os.Getenv("XDG_CONFIG_HOME") == "" {
		t.Errorf("GetConfigPath() = %v, should be under .config or XDG_CONFIG_HOME", path)
	}
}

func TestLoadConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `dest: /tmp/prow-artifacts
analyze_cmd: "echo test"
interactive: true
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := LoadConfigFile(configPath)
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v", err)
	}

	if cfg.Dest != "/tmp/prow-artifacts" {
		t.Errorf("LoadConfigFile().Dest = %v, want %v", cfg.Dest, "/tmp/prow-artifacts")
	}
	if cfg.AnalyzeCmd != "echo test" {
		t.Errorf("LoadConfigFile().AnalyzeCmd = %v, want %v", cfg.AnalyzeCmd, "echo test")
	}
	if !cfg.Interactive {
		t.Error("LoadConfigFile().Interactive = false, want true")
	}
}

func TestLoadConfigFile_NotExists(t *testing.T) {
	cfg, err := LoadConfigFile("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("LoadConfigFile() error = %v, should return empty config for missing file", err)
	}

	if cfg.Dest != "" {
		t.Errorf("LoadConfigFile().Dest = %v, want empty string for missing file", cfg.Dest)
	}
}

func TestLoadConfigFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := LoadConfigFile(configPath)
	if err == nil {
		t.Error("LoadConfigFile() should return error for invalid YAML")
	}
}

func TestLoadEnvConfig(t *testing.T) {
	// Save original env values
	origDest := os.Getenv("PROW_HELPER_DEST")
	origCmd := os.Getenv("PROW_HELPER_ANALYZE_CMD")
	origNtfy := os.Getenv("NTFY_CHANNEL")
	defer func() {
		os.Setenv("PROW_HELPER_DEST", origDest)
		os.Setenv("PROW_HELPER_ANALYZE_CMD", origCmd)
		os.Setenv("NTFY_CHANNEL", origNtfy)
	}()

	// Set test values
	os.Setenv("PROW_HELPER_DEST", "/env/path")
	os.Setenv("PROW_HELPER_ANALYZE_CMD", "env-command")
	os.Setenv("NTFY_CHANNEL", "test-channel")

	cfg := LoadEnvConfig()

	if cfg.Dest != "/env/path" {
		t.Errorf("LoadEnvConfig().Dest = %v, want %v", cfg.Dest, "/env/path")
	}
	if cfg.AnalyzeCmd != "env-command" {
		t.Errorf("LoadEnvConfig().AnalyzeCmd = %v, want %v", cfg.AnalyzeCmd, "env-command")
	}
	if cfg.NtfyChannel != "test-channel" {
		t.Errorf("LoadEnvConfig().NtfyChannel = %v, want %v", cfg.NtfyChannel, "test-channel")
	}
}

func TestLoadEnvConfig_Empty(t *testing.T) {
	// Save original env values
	origDest := os.Getenv("PROW_HELPER_DEST")
	origCmd := os.Getenv("PROW_HELPER_ANALYZE_CMD")
	origNtfy := os.Getenv("NTFY_CHANNEL")
	defer func() {
		os.Setenv("PROW_HELPER_DEST", origDest)
		os.Setenv("PROW_HELPER_ANALYZE_CMD", origCmd)
		os.Setenv("NTFY_CHANNEL", origNtfy)
	}()

	// Unset values
	os.Unsetenv("PROW_HELPER_DEST")
	os.Unsetenv("PROW_HELPER_ANALYZE_CMD")
	os.Unsetenv("NTFY_CHANNEL")

	cfg := LoadEnvConfig()

	if cfg.Dest != "" {
		t.Errorf("LoadEnvConfig().Dest = %v, want empty string", cfg.Dest)
	}
	if cfg.AnalyzeCmd != "" {
		t.Errorf("LoadEnvConfig().AnalyzeCmd = %v, want empty string", cfg.AnalyzeCmd)
	}
	if cfg.NtfyChannel != "" {
		t.Errorf("LoadEnvConfig().NtfyChannel = %v, want empty string", cfg.NtfyChannel)
	}
}

func TestMergeConfig(t *testing.T) {
	tests := []struct {
		name     string
		cli      *Config
		env      *Config
		file     *Config
		defaults *Config
		wantDest string
		wantCmd  string
	}{
		{
			name:     "all nil except defaults",
			cli:      nil,
			env:      nil,
			file:     nil,
			defaults: &Config{Dest: "default-dest", AnalyzeCmd: "default-cmd"},
			wantDest: "default-dest",
			wantCmd:  "default-cmd",
		},
		{
			name:     "file overrides defaults",
			cli:      nil,
			env:      nil,
			file:     &Config{Dest: "file-dest", AnalyzeCmd: ""},
			defaults: &Config{Dest: "default-dest", AnalyzeCmd: "default-cmd"},
			wantDest: "file-dest",
			wantCmd:  "default-cmd",
		},
		{
			name:     "env overrides file",
			cli:      nil,
			env:      &Config{Dest: "env-dest", AnalyzeCmd: ""},
			file:     &Config{Dest: "file-dest", AnalyzeCmd: "file-cmd"},
			defaults: &Config{Dest: "default-dest", AnalyzeCmd: "default-cmd"},
			wantDest: "env-dest",
			wantCmd:  "file-cmd",
		},
		{
			name:     "cli overrides all",
			cli:      &Config{Dest: "cli-dest", AnalyzeCmd: "cli-cmd"},
			env:      &Config{Dest: "env-dest", AnalyzeCmd: "env-cmd"},
			file:     &Config{Dest: "file-dest", AnalyzeCmd: "file-cmd"},
			defaults: &Config{Dest: "default-dest", AnalyzeCmd: "default-cmd"},
			wantDest: "cli-dest",
			wantCmd:  "cli-cmd",
		},
		{
			name:     "partial cli override",
			cli:      &Config{Dest: "cli-dest", AnalyzeCmd: ""},
			env:      &Config{Dest: "", AnalyzeCmd: "env-cmd"},
			file:     nil,
			defaults: &Config{Dest: "default-dest", AnalyzeCmd: "default-cmd"},
			wantDest: "cli-dest",
			wantCmd:  "env-cmd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeConfig(tt.cli, tt.env, tt.file, tt.defaults)
			if result.Dest != tt.wantDest {
				t.Errorf("MergeConfig().Dest = %v, want %v", result.Dest, tt.wantDest)
			}
			if result.AnalyzeCmd != tt.wantCmd {
				t.Errorf("MergeConfig().AnalyzeCmd = %v, want %v", result.AnalyzeCmd, tt.wantCmd)
			}
		})
	}
}
