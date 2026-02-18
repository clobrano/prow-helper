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
- **Watch Mode**: Poll running jobs until completion, then automatically download artifacts
- **ntfy.sh Notifications**: Receive push notifications on mobile devices via [ntfy.sh](https://ntfy.sh)

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

# Watch a running job until completion
prow-helper --watch <url>

# Watch job and analyze when complete
prow-helper --watch --analyze-cmd "claude 'analyze these failures'" <url>

# Watch with ntfy.sh notifications (for mobile alerts)
prow-helper --watch --ntfy-channel my-channel <url>

# Combine options
prow-helper --dest ~/artifacts --analyze-cmd "claude 'analyze these test failures'" --background <url>
```

### CLI Flags

| Flag | Description |
|------|-------------|
| `--dest` | Download destination directory (supports `~/` expansion) |
| `--analyze-cmd` | Command to run after download (receives artifact path as argument) |
| `--background` | Run in background and notify on completion |
| `--watch` | Poll job status until completion before downloading |
| `--ntfy-channel` | ntfy.sh channel for push notifications |
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

# ntfy.sh channel for push notifications (optional)
ntfy_channel: my-prow-notifications
```

### Environment Variables

```bash
export PROW_HELPER_DEST=~/my-artifacts
export PROW_HELPER_ANALYZE_CMD="claude 'analyze the Prow test artifacts'"
export NTFY_CHANNEL=my-prow-notifications
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
| 5 | Watch polling failed |
| 6 | Job completed with failure |

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

### Watch Mode

Monitor a running job and get notified when it completes:

```bash
# Watch until job completes, then notify
prow-helper --watch <url>

# Watch, download artifacts, and run analysis when complete
prow-helper --watch --analyze-cmd "claude 'analyze these failures'" <url>
```

The watch mode polls the job's `finished.json` every 15 minutes until the job completes.

### ntfy.sh Push Notifications

Receive notifications on your mobile device using [ntfy.sh](https://ntfy.sh):

1. Install the ntfy app on your phone
2. Subscribe to your chosen channel (e.g., `my-prow-notifications`)
3. Use the channel with prow-helper:

```bash
# One-time use
prow-helper --watch --ntfy-channel my-prow-notifications <url>

# Or configure permanently
echo "ntfy_channel: my-prow-notifications" >> ~/.config/prow-helper/config.yaml
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
