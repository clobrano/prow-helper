package config

import (
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

// Config holds the application configuration.
type Config struct {
	Dest       string `yaml:"dest"`        // Download destination directory
	AnalyzeCmd string `yaml:"analyze_cmd"` // Command to run after download
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		Dest:       ".",
		AnalyzeCmd: "",
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
	return &Config{
		Dest:       os.Getenv("PROW_HELPER_DEST"),
		AnalyzeCmd: os.Getenv("PROW_HELPER_ANALYZE_CMD"),
	}
}

// MergeConfig merges configurations with priority: cli > env > file > defaults.
// Non-empty values from higher priority configs override lower priority values.
func MergeConfig(cli, env, file, defaults *Config) *Config {
	result := &Config{}

	// Start with defaults
	if defaults != nil {
		result.Dest = defaults.Dest
		result.AnalyzeCmd = defaults.AnalyzeCmd
	}

	// Override with file config
	if file != nil {
		if file.Dest != "" {
			result.Dest = file.Dest
		}
		if file.AnalyzeCmd != "" {
			result.AnalyzeCmd = file.AnalyzeCmd
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
	}

	// Override with CLI config
	if cli != nil {
		if cli.Dest != "" {
			result.Dest = cli.Dest
		}
		if cli.AnalyzeCmd != "" {
			result.AnalyzeCmd = cli.AnalyzeCmd
		}
	}

	return result
}

// Load loads the full configuration by merging all sources.
// cliConfig should contain values from command-line flags (can be nil).
func Load(cliConfig *Config) (*Config, error) {
	defaults := DefaultConfig()
	envConfig := LoadEnvConfig()

	configPath := GetConfigPath()
	fileConfig, err := LoadConfigFile(configPath)
	if err != nil {
		return nil, err
	}

	return MergeConfig(cliConfig, envConfig, fileConfig, defaults), nil
}
