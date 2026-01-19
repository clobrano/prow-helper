# PRD: PROW Artifact Downloader & Analyzer

## 1. Introduction/Overview

**prow-helper** is a command-line tool designed to streamline the workflow of downloading PROW CI test artifacts and running automated analysis on them.

Currently, analyzing PROW test results requires multiple manual steps:
1. Copying the PROW test URL
2. Navigating to the artifacts page
3. Extracting the gsutil download command
4. Running the download command
5. Navigating to the downloaded folder
6. Running analysis tools manually

This tool consolidates these steps into a single command, allowing users to paste a PROW URL and automatically download artifacts and run a configurable analysis command in the background, with notification upon completion.

---

## 2. Goals

1. **Reduce manual effort**: Eliminate the need to manually construct gsutil commands from PROW URLs
2. **Streamline workflow**: Combine download and analysis into a single command invocation
3. **Background execution**: Allow users to start analysis and continue other work while waiting
4. **User notification**: Alert users when background analysis completes
5. **Flexible configuration**: Support multiple configuration methods for different use cases

---

## 3. User Stories

### US-1: Quick Analysis from PROW URL
> As a QE engineer, I want to copy a PROW test URL and run a single command to download and analyze the artifacts, so that I don't have to manually perform multiple steps.

### US-2: Background Processing
> As a developer, I want to start an artifact analysis in the background, so that I can continue working on other tasks while waiting for the download and analysis to complete.

### US-3: Completion Notification
> As a user, I want to receive a desktop notification when the analysis is complete, so that I know when results are ready without constantly checking.

### US-4: Configurable Analysis
> As a power user, I want to configure which analysis command runs after download, so that I can use different tools for different types of test failures.

### US-5: Organized Storage
> As a user who analyzes multiple test runs, I want artifacts to be organized by job name and build ID, so that I can easily find and reference them later.

---

## 4. Functional Requirements

### 4.1 URL Parsing

| ID | Requirement |
|----|-------------|
| FR-1 | The tool MUST accept PROW URLs in the format: `https://prow.ci.openshift.org/view/gs/<bucket>/<path>/<build-id>` |
| FR-2 | The tool MUST validate the URL format before attempting download |
| FR-3 | The tool MUST exit with an error and clear message if the URL format is invalid |
| FR-4 | The tool MUST extract the GCS bucket path from the PROW URL to construct the gsutil command |

### 4.2 Artifact Download

| ID | Requirement |
|----|-------------|
| FR-5 | The tool MUST use `gsutil -m cp -r` to download artifacts recursively |
| FR-6 | The tool MUST assume gsutil is installed and the user is authenticated |
| FR-7 | The tool MUST show download progress indication |
| FR-8 | The tool MUST organize downloaded artifacts in a folder named `<job-name>/<build-id>/` |
| FR-9 | If the destination folder already exists, the tool MUST prompt the user to choose: overwrite, skip, or create new timestamped folder |

### 4.3 Download Destination Configuration

| ID | Requirement |
|----|-------------|
| FR-10 | The tool MUST support configuring the download destination via command-line argument (`--dest`) |
| FR-11 | The tool MUST support configuring the download destination via environment variable (`PROW_HELPER_DEST`) |
| FR-12 | The tool MUST support configuring the download destination via configuration file following XDG Base Directory Specification (`$XDG_CONFIG_HOME/prow-helper/config.yaml`, defaulting to `~/.config/prow-helper/config.yaml`) |
| FR-13 | Configuration priority order MUST be: CLI argument > Environment variable > Config file > Current working directory |
| FR-14 | If no destination is configured, the tool MUST download to the current working directory |

### 4.4 Analysis Command Execution

| ID | Requirement |
|----|-------------|
| FR-15 | The tool MUST support running a configurable analysis command after download completes |
| FR-16 | The analysis command MUST be configurable via command-line argument (`--analyze-cmd`) |
| FR-17 | The analysis command MUST be configurable via environment variable (`PROW_HELPER_ANALYZE_CMD`) |
| FR-18 | The analysis command MUST be configurable via configuration file |
| FR-19 | The analysis command MUST receive the path to the downloaded artifacts folder as its argument |
| FR-20 | If the analysis command fails (non-zero exit code), the tool MUST exit with an error |
| FR-21 | If no analysis command is configured, the tool MUST only download artifacts (no error) |

### 4.5 Background Execution & Notifications

| ID | Requirement |
|----|-------------|
| FR-22 | The tool MUST support running in the background (e.g., via `--background` flag or `&`) |
| FR-23 | The tool MUST send a desktop notification when background processing completes |
| FR-24 | The notification MUST indicate success or failure status |
| FR-25 | The notification MUST include the job name or path to the artifacts |

### 4.6 Command-Line Interface

| ID | Requirement |
|----|-------------|
| FR-26 | The tool MUST be invoked as a CLI command (e.g., `prow-helper <url>`) |
| FR-27 | The tool MUST support `--help` to display usage information |
| FR-28 | The tool MUST support `--version` to display version information |
| FR-29 | The tool MUST process one URL at a time (no batch processing required) |

---

## 5. Non-Goals (Out of Scope)

The following are explicitly **not** in scope for this version:

1. **Batch processing**: Processing multiple URLs from a file or command line
2. **Download history**: Tracking previously downloaded artifacts
3. **Alternative download methods**: Using HTTP download instead of gsutil
4. **Authentication management**: Handling GCS authentication/credentials
5. **TUI or interactive mode**: Only CLI is required
6. **Artifact modification**: The tool should not modify downloaded artifacts
7. **Upload functionality**: The tool should not upload anything
8. **Support for other URL formats**: Only `prow.ci.openshift.org/view/gs/...` URLs

---

## 6. Design Considerations

### 6.1 CLI Usage Examples

```bash
# Basic usage - download and analyze
prow-helper https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296

# Specify destination
prow-helper --dest /tmp/prow-artifacts <url>

# Specify analysis command
prow-helper --analyze-cmd "claude-code --analyze" <url>

# Run in background
prow-helper --background <url>

# Combine options
prow-helper --dest ~/artifacts --analyze-cmd "./my-analyzer.sh" --background <url>
```

### 6.2 Configuration File Format

The configuration file follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html):

- **Location**: `$XDG_CONFIG_HOME/prow-helper/config.yaml`
- **Default**: `~/.config/prow-helper/config.yaml`

```yaml
# ~/.config/prow-helper/config.yaml
dest: ~/prow-artifacts
analyze_cmd: "claude-code --prompt 'Analyze these test artifacts'"
```

### 6.3 Environment Variables

| Variable | Description |
|----------|-------------|
| `PROW_HELPER_DEST` | Default download destination |
| `PROW_HELPER_ANALYZE_CMD` | Default analysis command |

---

## 7. Technical Considerations

### 7.1 Implementation Language
- **Go** is the preferred implementation language

### 7.2 Dependencies
- **gsutil**: Required for downloading artifacts from GCS
- Desktop notification library (e.g., `beeep` for Go)
- XDG-compliant config library (e.g., `adrg/xdg` for Go)

### 7.3 URL Parsing Logic

Given a URL like:
```
https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296
```

The tool should:
1. Validate URL matches pattern: `https://prow.ci.openshift.org/view/gs/<bucket>/<path>`
2. Extract bucket: `test-platform-results`
3. Extract path: `logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296`
4. Construct gsutil command: `gsutil -m cp -r gs://test-platform-results/logs/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296/ <dest>`
5. Extract job name: `periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview`
6. Extract build ID: `2013057817195319296`

### 7.4 Folder Organization

Downloaded artifacts should be stored as:
```
<dest>/<job-name>/<build-id>/
```

Example:
```
~/prow-artifacts/periodic-ci-openshift-release-master-nightly-4.22-e2e-metal-ovn-two-node-fencing-recovery-techpreview/2013057817195319296/
```

---

## 8. Success Metrics

| Metric | Target |
|--------|--------|
| Single command execution | User can download + analyze with one command |
| Time savings | Reduce manual steps from 6 to 1 |
| Background support | User can start analysis and continue other work |
| Notification delivery | User receives notification upon completion |

**Primary Success Criterion (from user):**
> "As a user I want to quickly copy the URL of a prow test and start an analysis in background, then be notified when it is ready"

---

## 9. Open Questions

1. **Notification mechanism**: Should the tool support configurable notification methods (desktop, email, webhook) in future versions?
2. **Analysis timeout**: Should there be a configurable timeout for long-running analysis commands?
3. **Partial downloads**: How should the tool handle interrupted downloads?

---

## 10. Appendix

### A. Example Workflow

```
User copies PROW URL from browser
    ↓
$ prow-helper --background https://prow.ci.openshift.org/view/gs/...
    ↓
Tool validates URL format
    ↓
Tool constructs gsutil command
    ↓
Tool downloads artifacts to ~/prow-artifacts/<job>/<build-id>/
    ↓
Tool runs configured analysis command with artifacts path
    ↓
Desktop notification: "PROW analysis complete: <job-name>"
```

### B. Error Scenarios

| Scenario | Expected Behavior |
|----------|-------------------|
| Invalid URL format | Exit with error: "Invalid PROW URL format. Expected: https://prow.ci.openshift.org/view/gs/..." |
| gsutil not found | Exit with error: "gsutil command not found. Please install Google Cloud SDK." |
| Download fails | Exit with error: "Failed to download artifacts: <gsutil error message>" |
| Analysis command fails | Exit with error: "Analysis failed with exit code <N>" |
| Destination folder exists | Prompt user: "Folder exists. [O]verwrite, [S]kip download, [N]ew timestamped folder?" |
