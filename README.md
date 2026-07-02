# prow-helper

A command-line tool for monitoring PROW CI jobs, downloading test artifacts, and running AI-powered analysis on them.

## Overview

Working with PROW CI typically means juggling browser tabs, polling job pages for completion, and manually downloading artifacts when things fail. **prow-helper** brings all of that into your terminal.

Use `--watch` to monitor running jobs — point it at a PROW job URL, a GitHub PR, or a Prow status page and it will poll until completion, showing live progress and sending desktop or mobile notifications when jobs finish.

Use `--download` to pull artifacts from GCS unconditionally, or `--analyze` to download (if needed) and hand them off to an AI tool like Claude or Gemini (or any other command, configured via `--analyze-cmd`) for automated failure analysis. Use `--analyze-on-failure` instead of `--analyze` to only bother downloading and analyzing jobs that actually failed.

These flags compose naturally — `--watch --analyze-cmd "..." --analyze-on-failure` watches a job, and only downloads and analyzes it if it fails.

## Features

### Monitoring & Notifications
- **Watch Mode** (`--watch`): Poll a single running job until completion with a live countdown display
- **Multi-Job Monitoring**: Pass a Prow status page URL to `--watch` and interactively select which jobs to monitor in a live status table
- **Desktop Notifications**: Get notified when jobs complete (Linux, macOS, Windows)
- **ntfy.sh Push Notifications**: Receive mobile alerts via [ntfy.sh](https://ntfy.sh)

### Download & Analysis
- **Download** (`--download`): Unconditionally download test artifacts from Google Cloud Storage
- **Analysis** (`--analyze`): Download (if needed) and run any command (Claude, Gemini, custom scripts) on the artifacts, configured via `--analyze-cmd`
- **Conditional Analysis** (`--analyze-on-failure`): Like `--analyze`, but only downloads/analyzes jobs that failed
- **Multiple Input Types**: Accepts direct PROW URLs, GitHub PR URLs, or any web page containing PROW links
- **Smart Job Discovery**: Automatically fetches associated PROW jobs from GitHub PRs via the Prow API, with interactive selection when multiple jobs are found
- **Parallel Downloads**: Uses `gsutil -m cp -r` for fast parallel downloads
- **Organized Storage**: Artifacts stored in structured folders: `<dest>/<job-name>/<build-id>/`
- **Conflict Resolution**: Prompts to overwrite, skip, or create timestamped folder when destination exists
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

prow-helper requires at least one action flag. Running it without flags prints the help message.

It accepts three types of input URLs: direct PROW URLs, GitHub PR URLs, and Prow status page URLs (or any web page containing PROW links). When multiple jobs are found (e.g., from a GitHub PR), prow-helper presents a numbered list so you can choose which one to work with.

### Action Flags

Each of these is independent — pass any combination that makes sense for your workflow. At least one is required.

| Flag | Description |
|------|-------------|
| `--watch` | Watch running jobs until completion and notify |
| `--download` | Unconditionally download test artifacts |
| `--analyze` | Download (if needed) and analyze artifacts with the configured `--analyze-cmd` |
| `--analyze-on-failure` | Like `--analyze`, but only downloads/analyzes if the job failed (mutually exclusive with `--analyze`) |

`--download` always downloads, regardless of whether the job passed or failed — it never checks job status. `--analyze` and `--analyze-on-failure` each trigger their own download automatically, so you don't need to pass `--download` alongside them (though you can — e.g. `--download --analyze-on-failure` downloads every run but only analyzes the failures).

### Quick Examples

```bash
# Watch a single PROW job until it completes
prow-helper --watch "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-name/12345"

# Watch a job from a GitHub PR (select which job interactively)
prow-helper --watch "https://github.com/openshift/cluster-network-operator/pull/42"

# Monitor multiple jobs from a Prow status page (interactive selector)
prow-helper --watch "https://prow.ci.openshift.org/?author=<your-username>"

# Download artifacts
prow-helper --download --dest ~/prow-artifacts <url>

# Download and analyze with Claude
prow-helper --analyze --analyze-cmd "claude 'analyze these test failures'" <url>

# Watch, then analyze when complete (downloads automatically)
prow-helper --watch --analyze --analyze-cmd "claude 'analyze'" <url>

# Watch, and only analyze if the job failed
prow-helper --watch --analyze-on-failure --analyze-cmd "claude 'analyze'" <url>

# Run in background with notification
prow-helper --watch --background <url>
```

### All Flags

| Flag | Description |
|------|-------------|
| `--watch` | Watch running jobs until completion |
| `--download` | Unconditionally download test artifacts |
| `--analyze` | Download (if needed) and analyze artifacts |
| `--analyze-on-failure` | Like `--analyze`, but only if the job failed |
| `--analyze-cmd` | Command to run during analysis (used with `--analyze` or `--analyze-on-failure`) |
| `--interval` | Polling interval for `--watch` status checks (default: 15m) |
| `--config` | Path to config file (default: `~/.config/prow-helper/config.yaml`) |
| `--dest` | Download destination directory (supports `~/` expansion) |
| `--ntfy-channel` | ntfy.sh channel for push notifications |
| `--background` | Run in background and notify on completion |
| `--help` | Display help information |
| `--version` | Display version information |

## Configuration

The config file provides default values for `--dest`, `--analyze-cmd`, and `--ntfy-channel` so you don't have to pass them on every invocation. You still need to specify an action flag (`--watch`, `--download`, `--analyze`, or `--analyze-on-failure`) on the command line — actions are never read from the config file.

### Configuration File

Default location: `~/.config/prow-helper/config.yaml` (follows XDG Base Directory Specification)

Use `--config` to point to a different file:

```bash
prow-helper --config ~/my-config.yaml --analyze <url>
```

```yaml
# Download destination
dest: ~/prow-artifacts

# Command to run during analysis (artifact path appended as last argument)
analyze_cmd: "claude 'analyze the Prow test artifacts contained in this folder'"

# Polling interval for --watch (default: 15m)
interval: 5m

# ntfy.sh channel for push notifications (optional)
ntfy_channel: my-prow-notifications
```

With this config, downloading and analyzing is just:

```bash
prow-helper --analyze <url>
```

Or watch a job, then analyze it when it completes:

```bash
prow-helper --watch --analyze <url>
```

### Environment Variables

```bash
export PROW_HELPER_DEST=~/my-artifacts
export PROW_HELPER_ANALYZE_CMD="claude 'analyze the Prow test artifacts'"
export PROW_HELPER_INTERVAL=5m
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

### Watch a Single Job

Monitor a running job and get notified when it completes. Works with any input type — PROW URLs, GitHub PR URLs, or web pages:

```bash
# Watch a PROW job
prow-helper --watch "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-name/12345"

# Watch a job from a GitHub PR (select which job interactively)
prow-helper --watch "https://github.com/openshift/cluster-network-operator/pull/42"
```

The watch mode polls the job's `finished.json` every 15 minutes until the job completes, showing a live countdown with elapsed time. Use `--interval` to change the polling frequency.

### Monitor Multiple Jobs

Pass a Prow status page URL to `--watch` to monitor multiple jobs at once:

```bash
prow-helper --watch "https://prow.ci.openshift.org/?author=<your-username>"

# Custom polling interval (default: 15 minutes)
prow-helper --watch --interval 5m "https://prow.ci.openshift.org/?author=clobrano"
```

prow-helper fetches all jobs from the status page via the `/prowjobs.js` API,
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

After confirming, prow-helper polls the selected jobs at the configured
interval and prints a live status table until all jobs complete.

### Download Artifacts

```bash
# Download artifacts from a completed job
prow-helper --download "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/my-job/54321"

# Download to a specific directory
prow-helper --download --dest ~/prow-artifacts <url>

# Download from a GitHub PR (select which job interactively)
prow-helper --download "https://github.com/openshift/cluster-network-operator/pull/42"
```

### Download and Analyze

`--analyze` downloads the artifacts (if not already present) and runs `--analyze-cmd` on them. It does not require `--download`.

```bash
# Download and analyze with Claude
prow-helper --analyze --analyze-cmd "claude 'analyze the Prow test artifacts contained in this folder'" <url>

# Or configure the analysis command once in ~/.config/prow-helper/config.yaml:
#   dest: ~/prow-artifacts
#   analyze_cmd: "claude 'analyze the Prow test artifacts contained in this folder'"
prow-helper --analyze <url>

# Only analyze if the job failed — skips download entirely when it passed
prow-helper --analyze-on-failure <url>
```

`--analyze-on-failure` also works with `--watch` (no separate status check needed, since watch already knows the outcome) or on its own against an already-finished job (prow-helper checks `finished.json` once before deciding).

### Combining Watch, Download, and Analyze

```bash
# Watch a job, then download artifacts when it completes
prow-helper --watch --download <url>

# Watch, then download and analyze (no need to also pass --download)
prow-helper --watch --analyze --analyze-cmd "claude 'analyze these failures'" <url>

# Watch, and only analyze if the job failed
prow-helper --watch --analyze-on-failure --analyze-cmd "claude 'analyze these failures'" <url>

# Always download every run, but only analyze the ones that failed
prow-helper --watch --download --analyze-on-failure --analyze-cmd "claude 'analyze these failures'" <url>
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

### Background Processing

```bash
prow-helper --download --background <url>
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
