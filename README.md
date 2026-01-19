# prow-helper

A command-line tool that automates downloading PROW CI test artifacts and running AI-powered analysis on them.

## Overview

Analyzing PROW test results typically requires multiple manual steps:
1. Copying the PROW test URL
2. Navigating to the artifacts page
3. Extracting the gsutil download command
4. Running the download command
5. Navigating to the downloaded folder
6. Running analysis tools manually

**prow-helper** consolidates these steps into a single command, allowing you to paste a PROW URL and automatically download artifacts and run AI analysis (with Claude, Gemini, or other tools) in the background, with notification upon completion.

## Features

- **Automated URL Handling**: Validates and parses PROW URLs, extracts GCS bucket and path, constructs gsutil commands automatically
- **Parallel Downloads**: Uses `gsutil -m cp -r` for fast parallel downloads from Google Cloud Storage
- **Organized Storage**: Artifacts stored in structured folders: `<dest>/<job-name>/<build-id>/`
- **Conflict Resolution**: Prompts to overwrite, skip, or create timestamped folder when destination exists
- **Flexible Configuration**: CLI flags, environment variables, and config file support
- **AI Analysis Integration**: Run Claude, Gemini, or other AI tools on downloaded artifacts
- **Background Processing**: Fork to background and receive desktop notification on completion

## Installation

### Prerequisites

- Go 1.21+
- [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) (gsutil) installed and authenticated
- Desktop notification support:
  - Linux: `notify-send` or D-Bus notification service
  - macOS: Notification Center
  - Windows: Windows Toast notifications

### Using go install

```bash
go install github.com/clobrano/prow-helper@latest
```

### Build from Source

```bash
git clone https://github.com/clobrano/prow-helper
cd prow-helper
go build
go install
```

## Usage

```bash
# Basic usage - download artifacts
prow-helper "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-name/12345"

# Download to specific destination
prow-helper --dest ~/prow-artifacts <url>

# Download and analyze with Claude (interactive session)
prow-helper --analyze-cmd "claude 'analyze the Prow test artifacts contained in this folder'" <url>

# Run in background with notification
prow-helper --background <url>

# Combine options
prow-helper --dest ~/artifacts --analyze-cmd "claude 'analyze these test failures'" --background <url>
```

### CLI Flags

| Flag | Description |
|------|-------------|
| `--dest` | Download destination directory (supports `~/` expansion) |
| `--analyze-cmd` | Command to run after download (receives artifact path as argument) |
| `--background` | Run in background and notify on completion |
| `--help` | Display help information |
| `--version` | Display version information |

## Configuration

### Configuration File

Location: `~/.config/prow-helper/config.yaml` (follows XDG Base Directory Specification)

```yaml
# Download destination
dest: ~/prow-artifacts

# Command to run after download (artifact path appended as last argument)
analyze_cmd: "claude 'analyze the Prow test artifacts contained in this folder'"
```

### Environment Variables

```bash
export PROW_HELPER_DEST=~/my-artifacts
export PROW_HELPER_ANALYZE_CMD="claude 'analyze the Prow test artifacts'"
```

### Configuration Priority

1. CLI flags (highest)
2. Environment variables
3. Config file
4. Defaults (current directory, no analysis command)

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Invalid PROW URL |
| 2 | Download failed |
| 3 | Analysis failed |
| 4 | Configuration error |

## Examples

### AI-Powered Analysis with Claude

```bash
# Configure once in ~/.config/prow-helper/config.yaml
# dest: ~/prow-artifacts
# analyze_cmd: "claude 'analyze the Prow test artifacts contained in this folder'"

prow-helper "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/my-job/54321"
# Downloads to ~/prow-artifacts/my-job/54321/ and starts Claude analysis
```

### Background Processing

```bash
prow-helper --background <url>
# Returns immediately, notification appears when download completes
```

### Handling Existing Folders

When artifacts already exist at the destination:
```
Folder exists. [O]verwrite, [S]kip download, [N]ew timestamped folder?
```

## Development

```bash
# Build
go build

# Run tests
go test ./...

# Install locally
go install
```

## License

MIT
