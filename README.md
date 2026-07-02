# prow-helper

A command-line tool for monitoring PROW CI jobs, downloading test artifacts, and running AI-powered analysis on them.

## Overview

Working with PROW CI typically means juggling browser tabs, polling job pages for completion, and manually downloading artifacts when things fail. **prow-helper** brings all of that into your terminal.

Use `--watch` to monitor running jobs — point it at a PROW job URL, a GitHub PR, or a Prow status page and it will poll until completion, showing live progress and sending desktop or mobile notifications when jobs finish.

Use `--download` to pull artifacts from GCS, and pair it with `--analyze-cmd` to hand them off to an AI tool like Claude or Gemini (or any other command) for automated failure analysis.

These flags compose naturally — `--watch --download --analyze-cmd "..."` watches a job, downloads artifacts when it completes, and runs analysis in one go.

## Features

### Monitoring & Notifications
- **Watch Mode** (`--watch`): Poll a single running job until completion with a live countdown display
- **Multi-Job Monitoring**: Pass a Prow status page URL to `--watch` and interactively select which jobs to monitor in a live status table
- **Desktop Notifications**: Get notified when jobs complete (Linux, macOS, Windows)
- **ntfy.sh Push Notifications**: Receive mobile alerts via [ntfy.sh](https://ntfy.sh)

### Download & Analysis
- **Download** (`--download`): Download test artifacts from Google Cloud Storage
- **Analysis** (`--analyze-cmd`): Run any command (Claude, Gemini, custom scripts) on downloaded artifacts; requires `--download`
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

| Flag | Description |
|------|-------------|
| `--watch` | Watch running jobs until completion and notify |
| `--download` | Download test artifacts |
| `--analyze-cmd` | Run a command on downloaded artifacts (requires `--download`) |
| `--only-on-failure` | Only download/analyze artifacts if the job failed (requires `--download`) |

These can be combined: `--watch --download` watches until the job completes, then downloads. Add `--analyze-cmd` to also run analysis after download. Add `--only-on-failure` to skip the download (and analysis) entirely when the job passed — handy when you only care about investigating failures.

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
prow-helper --download --analyze-cmd "claude 'analyze these test failures'" <url>

# Watch, then download and analyze when complete
prow-helper --watch --download --analyze-cmd "claude 'analyze'" <url>

# Run in background with notification
prow-helper --watch --background <url>
```

### All Flags

| Flag | Description |
|------|-------------|
| `--watch` | Watch running jobs until completion |
| `--download` | Download test artifacts |
| `--analyze-cmd` | Command to run on downloaded artifacts (requires `--download`) |
| `--only-on-failure` | Only download/analyze artifacts if the job failed (requires `--download`) |
| `--interval` | Polling interval for `--watch` status checks (default: 15m) |
| `--config` | Path to config file (default: `~/.config/prow-helper/config.yaml`) |
| `--dest` | Download destination directory (supports `~/` expansion) |
| `--ntfy-channel` | ntfy.sh channel for push notifications |
| `--background` | Run in background and notify on completion |
| `--help` | Display help information |
| `--version` | Display version information |

## Configuration

The config file provides default values for `--dest`, `--analyze-cmd`, and `--ntfy-channel` so you don't have to pass them on every invocation. You still need to specify an action flag (`--watch` or `--download`) on the command line.

### Configuration File

Default location: `~/.config/prow-helper/config.yaml` (follows XDG Base Directory Specification)

Use `--config` to point to a different file:

```bash
prow-helper --config ~/my-config.yaml --download <url>
```

```yaml
# Download destination
dest: ~/prow-artifacts

# Command to run after download (artifact path appended as last argument)
analyze_cmd: "claude 'analyze the Prow test artifacts contained in this folder'"

# Polling interval for --watch (default: 15m)
interval: 5m

# ntfy.sh channel for push notifications (optional)
ntfy_channel: my-prow-notifications

# Only download/analyze artifacts if the job failed (default: false)
only_on_failure: true
```

With this config, downloading and analyzing is just:

```bash
prow-helper --download <url>
```

Or watch a job, then download and analyze when it completes:

```bash
prow-helper --watch --download <url>
```

### Environment Variables

```bash
export PROW_HELPER_DEST=~/my-artifacts
export PROW_HELPER_ANALYZE_CMD="claude 'analyze the Prow test artifacts'"
export PROW_HELPER_INTERVAL=5m
export NTFY_CHANNEL=my-prow-notifications
export PROW_HELPER_ONLY_ON_FAILURE=true
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

`--analyze-cmd` runs any command on the downloaded artifacts. It requires `--download`.

```bash
# Download and analyze with Claude
prow-helper --download --analyze-cmd "claude 'analyze the Prow test artifacts contained in this folder'" <url>

# Or configure the analysis command once in ~/.config/prow-helper/config.yaml:
#   dest: ~/prow-artifacts
#   analyze_cmd: "claude 'analyze the Prow test artifacts contained in this folder'"
prow-helper --download <url>
```

### Combining Watch + Download

```bash
# Watch a job, then download artifacts when it completes
prow-helper --watch --download <url>

# Watch, download, and analyze
prow-helper --watch --download --analyze-cmd "claude 'analyze these failures'" <url>

# Watch, and only download + analyze if the job failed
prow-helper --watch --download --analyze-cmd "claude 'analyze these failures'" --only-on-failure <url>
```

`--only-on-failure` also works with `--download` alone (no `--watch`): prow-helper checks the job's current `finished.json` once before downloading, and skips the download (and analysis) if the job passed.

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
