# prow-helper

A command-line tool for monitoring PROW CI jobs, downloading test artifacts, and running AI-powered analysis on them.

## Overview

Working with PROW CI typically means juggling browser tabs, polling job pages for completion, and manually downloading artifacts when things fail. **prow-helper** brings all of that into a single command.

**Watch and monitor jobs** — Point prow-helper at a running job or a Prow status page and it will poll until completion, showing live progress and sending desktop or mobile notifications when jobs finish.

**Download and analyze artifacts** — Once a job completes (or for jobs already finished), prow-helper downloads artifacts from GCS and optionally hands them off to an AI tool like Claude or Gemini for automated failure analysis.

It accepts PROW URLs, GitHub PR URLs, or any web page containing PROW links, so you can work from wherever you are.

## Features

### Monitoring & Notifications
- **Watch Mode**: Poll a single running job until completion with a live countdown display; proceeds to download and analyze artifacts when `--analyze-cmd` is set
- **Monitor Command**: Fetch all jobs from a Prow status page, interactively select which to watch, and track their progress in a live status table
- **Desktop Notifications**: Get notified when jobs complete (Linux, macOS, Windows)
- **ntfy.sh Push Notifications**: Receive mobile alerts via [ntfy.sh](https://ntfy.sh)

### Download & Analysis
- **Multiple Input Types**: Accepts direct PROW URLs, GitHub PR URLs, or any web page containing PROW links
- **Smart Job Discovery**: Automatically fetches associated PROW jobs from GitHub PRs via the Prow API, with interactive selection when multiple jobs are found
- **Parallel Downloads**: Uses `gsutil -m cp -r` for fast parallel downloads from Google Cloud Storage
- **Organized Storage**: Artifacts stored in structured folders: `<dest>/<job-name>/<build-id>/`
- **Conflict Resolution**: Prompts to overwrite, skip, or create timestamped folder when destination exists
- **AI Analysis Integration**: Run Claude, Gemini, or other AI tools on downloaded artifacts
- **Background Processing**: Fork to background and receive desktop notification on completion

### Configuration
- **Flexible Configuration**: CLI flags, environment variables, and config file support

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

prow-helper accepts three types of input URLs:

```bash
# Direct PROW URL - download artifacts directly
prow-helper "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-name/12345"

# GitHub PR URL - fetches associated PROW jobs, lets you pick which one
prow-helper "https://github.com/openshift/cluster-network-operator/pull/42"

# Any web page - scans for embedded PROW links and lets you select
prow-helper "https://example.com/page-with-prow-links"
```

When multiple PROW jobs are found (e.g., from a GitHub PR), prow-helper presents a numbered list so you can choose which job's artifacts to download.

### Common Options

```bash
# Watch a running job until completion, then notify
prow-helper --watch <url>

# Watch and get mobile notifications when done
prow-helper --watch --ntfy-channel my-channel <url>

# Watch, download, and analyze when complete
prow-helper --watch --analyze-cmd "claude 'analyze these failures'" <url>

# Monitor multiple jobs from a Prow status page
prow-helper monitor "https://prow.ci.openshift.org/?author=<your-username>"

# Download artifacts to a specific destination
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
| `--watch` | Poll job status until completion before downloading |
| `--ntfy-channel` | ntfy.sh channel for push notifications |
| `monitor --interval` | Polling interval for `monitor` status checks (default: 15m) |
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

### Watch Mode

Monitor a running job and get notified when it completes:

```bash
# Watch until job completes, then notify with pass/fail status
prow-helper --watch <url>

# Watch, then download artifacts and run analysis when complete
prow-helper --watch --analyze-cmd "claude 'analyze these failures'" <url>
```

The watch mode polls the job's `finished.json` every 15 minutes until the job completes, showing a live countdown with elapsed time. On its own, `--watch` reports the result and exits. Combined with `--analyze-cmd`, it continues to download artifacts and run the analysis command.

### Monitor Command

Watch multiple jobs from a Prow status page in one shot:

```bash
prow-helper monitor "https://prow.ci.openshift.org/?author=<your-username>"
```

The command fetches all jobs from the status page via the `/prowjobs.js` API,
then opens an interactive selector:

| Key | Action |
|-----|--------|
| Type | Filter by substring (job name, state, …) |
| ↑ / ↓ | Move cursor |
| `Space` | Toggle job under cursor |
| `Ctrl+A` | Select / deselect all visible jobs |
| `Ctrl+R` | Refresh job list from API (preserves selections) |
| `Enter` | Confirm selection and start monitoring |
| `Esc` | Clear search (first press) or cancel (second press) |

After confirming, prow-helper polls the selected jobs at a configurable
interval and prints a live status table until all jobs complete.

```bash
# Custom polling interval (default: 15 minutes)
prow-helper monitor --interval 5m "https://prow.ci.openshift.org/?author=clobrano"
```

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

### GitHub PR Workflow

```bash
# Pass a GitHub PR URL - prow-helper queries the Prow API for associated jobs
prow-helper "https://github.com/openshift/cluster-network-operator/pull/42"
# Lists all CI jobs for the PR, select one, and download its artifacts

# Watch a PR's CI job until it finishes, then analyze
prow-helper --watch "https://github.com/openshift/cluster-network-operator/pull/42"
```

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
