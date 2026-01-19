# Tasks: PROW Artifact Downloader & Analyzer

Generated from: `prd-prow-artifact-analyzer.md`

---

## Relevant Files

- `cmd/prow-helper/main.go` - Application entry point
- `cmd/prow-helper/root.go` - Cobra root command definition with CLI flags
- `cmd/prow-helper/root_test.go` - Tests for CLI argument parsing
- `internal/config/config.go` - Configuration loading and merging logic
- `internal/config/config_test.go` - Tests for configuration system
- `internal/parser/url.go` - PROW URL parsing and validation
- `internal/parser/url_test.go` - Tests for URL parsing
- `internal/downloader/downloader.go` - gsutil wrapper for artifact download
- `internal/downloader/downloader_test.go` - Tests for downloader
- `internal/analyzer/analyzer.go` - Analysis command execution
- `internal/analyzer/analyzer_test.go` - Tests for analyzer
- `internal/notifier/notifier.go` - Desktop notification functionality
- `internal/notifier/notifier_test.go` - Tests for notifier
- `go.mod` - Go module definition
- `go.sum` - Dependency checksums
- `Makefile` - Build, test, and install targets

### Notes

- Each task/sub-task MUST follow the Test Driven Development approach, unless there is nothing to test (e.g., creating infrastructure)
- Unit tests should be placed alongside the code files they are testing (e.g., `url.go` and `url_test.go` in the same directory)
- Task list includes 2 integration/validation phases: after Task 4.0 (download flow) and after Task 8.0 (full workflow)

---

## Tasks

- [x] 1.0 Project Setup & Infrastructure
  - [x] 1.1 Initialize Go module with `go mod init github.com/clobrano/prow-helper`
  - [x] 1.2 Create directory structure: `cmd/prow-helper/`, `internal/config/`, `internal/parser/`, `internal/downloader/`, `internal/analyzer/`, `internal/notifier/`
  - [x] 1.3 Add dependencies: `cobra` (CLI), `yaml.v3` (config parsing), `adrg/xdg` (XDG paths), `gen2brain/beeep` (notifications)
  - [x] 1.4 Create `Makefile` with targets: `build`, `test`, `install`, `clean`

- [x] 2.0 URL Parsing & Validation
  - [x] 2.1 Write tests for PROW URL validation (valid URLs, invalid formats, edge cases)
  - [x] 2.2 Implement `ValidateURL(url string) error` function
  - [x] 2.3 Write tests for metadata extraction (bucket, path, job name, build ID)
  - [x] 2.4 Implement `ParseURL(url string) (*ProwMetadata, error)` returning structured metadata
  - [x] 2.5 Write tests for gsutil command construction
  - [x] 2.6 Implement `BuildGsutilCommand(metadata *ProwMetadata, dest string) string`

- [x] 3.0 Configuration System
  - [x] 3.1 Define `Config` struct with fields: `Dest`, `AnalyzeCmd`
  - [x] 3.2 Write tests for XDG config file path resolution
  - [x] 3.3 Implement `GetConfigPath() string` using XDG Base Directory Specification
  - [x] 3.4 Write tests for YAML config file loading
  - [x] 3.5 Implement `LoadConfigFile(path string) (*Config, error)`
  - [x] 3.6 Write tests for environment variable loading
  - [x] 3.7 Implement `LoadEnvConfig() *Config` for `PROW_HELPER_DEST` and `PROW_HELPER_ANALYZE_CMD`
  - [x] 3.8 Write tests for configuration merging with priority order
  - [x] 3.9 Implement `MergeConfig(cli, env, file, defaults *Config) *Config` with priority: CLI > Env > File > Defaults

- [x] 4.0 Artifact Download
  - [x] 4.1 Write tests for destination path construction (`<dest>/<job-name>/<build-id>/`)
  - [x] 4.2 Implement `BuildDestinationPath(baseDest string, metadata *ProwMetadata) string`
  - [x] 4.3 Write tests for folder existence check and conflict detection
  - [x] 4.4 Implement `CheckDestinationConflict(path string) (exists bool, err error)`
  - [x] 4.5 Implement user prompt for conflict resolution: Overwrite, Skip, New timestamped folder
  - [x] 4.6 Write tests for gsutil command execution (mock external command)
  - [x] 4.7 Implement `Download(gsutilCmd string) error` with progress output passthrough
  - [x] 4.8 Implement gsutil availability check with clear error message

- [ ] **CHECKPOINT: Manual Integration Test - Download Flow**
  - [ ] 4.9 Manually test URL parsing with a real PROW URL
  - [ ] 4.10 Manually test artifact download to a local folder

- [x] 5.0 Analysis Command Execution
  - [x] 5.1 Write tests for analysis command parsing (handle quotes, arguments)
  - [x] 5.2 Implement `ParseAnalyzeCommand(cmd string) (name string, args []string)`
  - [x] 5.3 Write tests for command execution with artifacts path as argument
  - [x] 5.4 Implement `RunAnalysis(cmd string, artifactsPath string) error`
  - [x] 5.5 Write tests for error handling (non-zero exit code)
  - [x] 5.6 Implement proper error propagation with exit code information

- [x] 6.0 Background Execution & Notifications
  - [x] 6.1 Write tests for notification message formatting
  - [x] 6.2 Implement `Notify(title, message string, success bool) error` using beeep or notify-send fallback
  - [x] 6.3 Implement `--background` flag that forks process and returns immediately
  - [x] 6.4 Integrate notification calls for success and failure scenarios
  - [x] 6.5 Ensure notification includes job name and status

- [x] 7.0 CLI Interface & Main Orchestration
  - [x] 7.1 Set up cobra root command in `cmd/prow-helper/root.go`
  - [x] 7.2 Add CLI flags: `--dest`, `--analyze-cmd`, `--background`, `--version`, `--help`
  - [x] 7.3 Implement `--version` flag to display version information
  - [x] 7.4 Wire configuration loading (merge CLI flags, env, config file)
  - [x] 7.5 Implement main workflow in `Run()`: validate URL → load config → download → analyze → notify
  - [x] 7.6 Add proper exit codes: 0 (success), 1 (invalid URL), 2 (download failed), 3 (analysis failed)
  - [x] 7.7 Implement clear error messages matching PRD error scenarios

- [x] 8.0 Integration Testing & Final Validation
  - [x] 8.1 Write integration test with mocked gsutil command
  - [x] 8.2 Write integration test for config file loading from XDG path
  - [ ] 8.3 Manual test: full workflow with real PROW URL (download + analyze)
  - [ ] 8.4 Manual test: background mode with desktop notification
  - [ ] 8.5 Manual test: configuration via environment variables
  - [ ] 8.6 Manual test: folder conflict prompt (Overwrite/Skip/New)
  - [x] 8.7 Code review: check for proper error handling and edge cases
  - [ ] 8.8 Update README with usage examples (optional, if requested)
