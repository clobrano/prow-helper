package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	Dest          string        `yaml:"dest"`            // Download destination directory
	AnalyzeCmd    string        `yaml:"analyze_cmd"`     // Command to run after download
	NtfyChannel   string        `yaml:"ntfy_channel"`    // ntfy.sh channel for notifications
	Interval      time.Duration `yaml:"interval"`        // Polling interval for --watch
	OnlyOnFailure bool          `yaml:"only_on_failure"` // Only download/analyze when the job failed
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Dest:          ".",
		AnalyzeCmd:    "",
		NtfyChannel:   "",
		Interval:      15 * time.Minute,
		OnlyOnFailure: false,
	}
}

// GetConfigPath returns the XDG-compliant config file path.
// Uses $XDG_CONFIG_HOME/prow-helper/config.yaml, defaulting to ~/.config/prow-helper/config.yaml
func GetConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "prow-helper", "config.yaml")
}

// LoadConfigFile loads configuration from a YAML file.
// Returns an empty Config if the file doesn't exist.
func LoadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadEnvConfig loads configuration from environment variables.
func LoadEnvConfig() *Config {
	cfg := &Config{
		Dest:        os.Getenv("PROW_HELPER_DEST"),
		AnalyzeCmd:  os.Getenv("PROW_HELPER_ANALYZE_CMD"),
		NtfyChannel: os.Getenv("NTFY_CHANNEL"),
	}
	if v := os.Getenv("PROW_HELPER_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.Interval = d
		}
	}
	if v := os.Getenv("PROW_HELPER_ONLY_ON_FAILURE"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.OnlyOnFailure = b
		}
	}
	return cfg
}

// MergeConfig merges configurations with priority: cli > env > file > defaults.
// Non-empty values from higher priority configs override lower priority values.
// OnlyOnFailure has no "unset" representation (its zero value, false, is also
// its default), so it is merged by OR: it is enabled if any source enables it.
func MergeConfig(cli, env, file, defaults *Config) *Config {
	result := &Config{}

	// Start with defaults
	if defaults != nil {
		result.Dest = defaults.Dest
		result.AnalyzeCmd = defaults.AnalyzeCmd
		result.NtfyChannel = defaults.NtfyChannel
		result.Interval = defaults.Interval
		result.OnlyOnFailure = defaults.OnlyOnFailure
	}

	// Override with file config
	if file != nil {
		if file.Dest != "" {
			result.Dest = file.Dest
		}
		if file.AnalyzeCmd != "" {
			result.AnalyzeCmd = file.AnalyzeCmd
		}
		if file.NtfyChannel != "" {
			result.NtfyChannel = file.NtfyChannel
		}
		if file.Interval != 0 {
			result.Interval = file.Interval
		}
		if file.OnlyOnFailure {
			result.OnlyOnFailure = true
		}
	}

	// Override with env config
	if env != nil {
		if env.Dest != "" {
			result.Dest = env.Dest
		}
		if env.AnalyzeCmd != "" {
			result.AnalyzeCmd = env.AnalyzeCmd
		}
		if env.NtfyChannel != "" {
			result.NtfyChannel = env.NtfyChannel
		}
		if env.Interval != 0 {
			result.Interval = env.Interval
		}
		if env.OnlyOnFailure {
			result.OnlyOnFailure = true
		}
	}

	// Override with CLI config
	if cli != nil {
		if cli.Dest != "" {
			result.Dest = cli.Dest
		}
		if cli.AnalyzeCmd != "" {
			result.AnalyzeCmd = cli.AnalyzeCmd
		}
		if cli.NtfyChannel != "" {
			result.NtfyChannel = cli.NtfyChannel
		}
		if cli.Interval != 0 {
			result.Interval = cli.Interval
		}
		if cli.OnlyOnFailure {
			result.OnlyOnFailure = true
		}
	}

	return result
}

// Load loads the full configuration by merging all sources.
// cliConfig should contain values from command-line flags (can be nil).
// configPath overrides the default config file location when non-empty.
func Load(cliConfig *Config, configPath string) (*Config, error) {
	defaults := DefaultConfig()
	envConfig := LoadEnvConfig()

	if configPath == "" {
		configPath = GetConfigPath()
	}
	fileConfig, err := LoadConfigFile(configPath)
	if err != nil {
		return nil, err
	}

	return MergeConfig(cliConfig, envConfig, fileConfig, defaults), nil
}
