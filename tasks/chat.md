# PRD Clarifying Questions: PROW Artifact Downloader & Analyzer

Please answer the following questions to help me create a detailed PRD. You can respond inline after each question or use the letter/number to indicate your choice.

---

## 1. Problem/Goal

**1.1 What is the primary pain point you're experiencing today?**
- a) Manually copying URLs and running gsutil commands is tedious
- b) Forgetting where artifacts were downloaded
- c) Needing to run analysis commands manually after download
- d) All of the above
- e) Other (please describe): ___

D

**1.2 How frequently do you need to download and analyze PROW artifacts?**
- a) Multiple times per day
- b) A few times per week
- c) Occasionally (a few times per month)
- d) Other: ___

C

---

## 2. Input/URL Handling

**2.1 What types of PROW URLs should the tool accept?**
- a) Only the prow.ci.openshift.org/view/gs/... format (as in your example)
- b) Also the direct gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com URLs
- c) Also the gs:// bucket paths directly
- d) All of the above

As far as I know, only "A"

**2.2 Should the tool validate the URL before attempting download?**
- a) Yes, fail fast if URL format is invalid
- b) Yes, but just warn and attempt anyway
- c) No validation needed

A

---

## 3. Download Destination

**3.1 How should the download destination be configured?**
- a) Command-line argument (e.g., `--dest /path/to/folder`)
- b) Environment variable (e.g., `PROW_ARTIFACTS_DIR`)
- c) Configuration file (e.g., `~/.prow-helper.yaml`)
- d) All of the above with priority order
- e) Other: ___

D, but if none is configured, just download in the current working directory

**3.2 How should downloaded artifacts be organized?**
- a) Use the job name and build ID as folder name (e.g., `periodic-ci-openshift-release-master.../2013057817195319296/`)
- b) Use a custom naming scheme (please describe): ___
- c) Just download to the specified folder directly
- d) Let me choose per download

A

**3.3 What should happen if the destination folder already exists with previous artifacts?**
- a) Overwrite existing files
- b) Skip download and reuse existing
- c) Create a new timestamped folder
- d) Ask the user what to do
- e) Configurable behavior

D

---

## 4. Analysis Command

**4.1 What kind of analysis command do you want to run?**
- a) A shell command/script (e.g., `./analyze.sh`)
- b) An AI agent (e.g., Claude Code, custom LLM agent)
- c) A specific tool (please name it): ___
- d) Should be configurable to run any command

D

**4.2 How should the analysis command be configured?**
- a) Command-line argument (e.g., `--analyze-cmd "..."`)
- b) Configuration file
- c) Environment variable
- d) All of the above

D

**4.3 Should the analysis command receive any specific arguments?**
- a) Just the path to the downloaded artifacts folder
- b) The original PROW URL as well
- c) Parsed metadata (job name, build ID, etc.)
- d) Other: ___

A

**4.4 What should happen if the analysis command fails?**
- a) Exit with error
- b) Log the error and continue
- c) Retry a configurable number of times
- d) Other: ___

A

---

## 5. User Interface

**5.1 What interface should the tool provide?**
- a) CLI only (command-line tool)
- b) CLI with optional interactive mode
- c) TUI (terminal user interface)
- d) Other: ___

A

**5.2 Should the tool support batch processing (multiple URLs)?**
- a) Yes, from a file containing URLs
- b) Yes, from command-line arguments
- c) Yes, both
- d) No, single URL at a time is sufficient

D

---

## 6. Technical Preferences

**6.1 What programming language do you prefer for implementation?**
- a) Go
- b) Python
- c) Bash/Shell script
- d) Rust
- e) No preference

A

**6.2 Do you need the tool to work without gsutil installed?**
- a) No, gsutil will always be available
- b) Yes, provide a fallback method
- c) Yes, don't use gsutil at all (use direct HTTP download)

A

**6.3 Should the tool handle authentication to GCS?**
- a) No, assume user is already authenticated
- b) Yes, prompt for credentials if needed
- c) Yes, support service account configuration

A

---

## 7. Additional Features

**7.1 Would you like any of these optional features?**
- a) Progress indication during download
- b) Logging to file
- c) Dry-run mode (show what would be done without doing it)
- d) Quiet mode (minimal output)
- e) All of the above
- f) Other: ___

A

**7.2 Should the tool store a history of downloaded artifacts?**
- a) Yes, in a local database/file
- b) No, not needed
- c) Optional, configurable

B

---

## 8. Success Criteria

**8.1 What would make this tool successful for you?**
(Please describe in your own words)

As a user I want to quickly copy the URL of a prow test and start an analysis in background, then be notified when it is ready

---

## 9. Non-Goals / Out of Scope

**9.1 Is there anything this tool should explicitly NOT do?**
(Please describe, e.g., "Should not upload anything", "Should not modify artifacts")

N/A

---

## 10. Open Questions

**Do you have any other requirements, constraints, or preferences I should know about?**

N/A
---

*Please answer the questions above, and I'll generate a comprehensive PRD based on your responses.*
