package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/clobrano/prow-helper/internal/analyzer"
	"github.com/clobrano/prow-helper/internal/config"
	"github.com/clobrano/prow-helper/internal/downloader"
	"github.com/clobrano/prow-helper/internal/gitdetect"
	"github.com/clobrano/prow-helper/internal/notifier"
	"github.com/clobrano/prow-helper/internal/output"
	"github.com/clobrano/prow-helper/internal/parser"
	"github.com/clobrano/prow-helper/internal/prowapi"
	"github.com/clobrano/prow-helper/internal/resolver"
	"github.com/clobrano/prow-helper/internal/selector"
	"github.com/clobrano/prow-helper/internal/watcher"
)

// Exit codes
const (
	ExitSuccess        = 0
	ExitInvalidURL     = 1
	ExitDownloadFailed = 2
	ExitAnalysisFailed = 3
	ExitConfigError    = 4
	ExitWatchFailed    = 5
	ExitJobFailed      = 6
	ExitInterrupted    = 130 // 128 + SIGINT, the conventional interrupt exit code
)

var (
	// CLI flags
	flagDest             string
	flagAnalyzeCmd       string
	flagBackground       bool
	flagNotifyComplete   bool // Internal flag set by background mode
	flagWatch            bool
	flagDownload         bool
	flagAnalyze          bool
	flagAnalyzeOnFailure bool
	flagNtfyChannel      string
	flagInterval         time.Duration
	flagConfig           string
	flagOnConflict       string
)

// anyDownloadAction reports whether any action flag may trigger an artifact
// download (--analyze-on-failure only does so conditionally, once the job's
// pass/fail status is known).
func anyDownloadAction() bool {
	return flagDownload || flagAnalyze || flagAnalyzeOnFailure
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "prow-helper [flags] <url>",
	Short: "Monitor PROW CI jobs, download test artifacts, and run analysis",
	Long: `prow-helper monitors PROW CI jobs, downloads test artifacts, and runs
analysis on them.

It accepts a PROW test URL, a GitHub pull request URL, a Prow status page URL,
or any page containing prow job links.

When using --watch without a URL inside a git repository that has an open
pull request, prow-helper will auto-detect the PR and watch it.

At least one action flag is required:
  --watch               Watch running jobs until completion
  --download            Download test artifacts
  --analyze             Download (if needed) and analyze artifacts
  --analyze-on-failure  Like --analyze, but only if the job failed

Examples:
  # Auto-detect PR from current git directory
  prow-helper --watch

  # Watch a single job
  prow-helper --watch https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-name/12345

  # Watch jobs from a GitHub PR
  prow-helper --watch https://github.com/openshift/cno/pull/42

  # Monitor multiple jobs from a status page (interactive selector)
  prow-helper --watch "https://prow.ci.openshift.org/?author=your-username"

  # Download artifacts
  prow-helper --download --dest ~/artifacts <url>

  # Watch, then analyze (downloads automatically)
  prow-helper --watch --analyze-cmd "claude 'analyze'" --analyze <url>

  # Watch, then analyze only if the job failed
  prow-helper --watch --analyze-cmd "claude 'analyze'" --analyze-on-failure <url>`,
	Args: cobra.MaximumNArgs(1),
	RunE: runMain,
}

func init() {
	rootCmd.Flags().BoolVar(&flagWatch, "watch", false, "Watch running jobs until completion")
	rootCmd.Flags().BoolVar(&flagDownload, "download", false, "Download test artifacts")
	rootCmd.Flags().BoolVar(&flagAnalyze, "analyze", false, "Download (if needed) and analyze artifacts with the configured analyze command")
	rootCmd.Flags().BoolVar(&flagAnalyzeOnFailure, "analyze-on-failure", false, "Like --analyze, but only download/analyze if the job failed")
	rootCmd.Flags().StringVar(&flagAnalyzeCmd, "analyze-cmd", "", "Command to run during analysis (used with --analyze or --analyze-on-failure)")
	rootCmd.Flags().DurationVar(&flagInterval, "interval", 0, "Polling interval for --watch status checks (default: 15m)")
	rootCmd.Flags().StringVar(&flagConfig, "config", "", "Path to config file (default: ~/.config/prow-helper/config.yaml)")
	rootCmd.Flags().StringVar(&flagDest, "dest", "", "Download destination directory")
	rootCmd.Flags().StringVar(&flagOnConflict, "on-conflict", "", "How to handle an existing download folder: prompt|overwrite|skip|new (default: prompt)")
	rootCmd.Flags().StringVar(&flagNtfyChannel, "ntfy-channel", "", "ntfy.sh channel for notifications")
	rootCmd.Flags().BoolVar(&flagBackground, "background", false, "Run in background and notify when done")
	rootCmd.Flags().BoolVar(&flagNotifyComplete, "notify-on-complete", false, "Internal flag for background mode notifications")
	rootCmd.Flags().MarkHidden("notify-on-complete") // Hide from help output
	rootCmd.Version = Version
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runMain(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		if !flagWatch {
			return cmd.Help()
		}
		// --watch without URL: try to auto-detect PR from git directory
		detectedURL, err := gitdetect.DetectPRURL()
		if err != nil {
			fmt.Fprintf(os.Stderr, "No URL provided and could not auto-detect PR: %v\n\n", err)
			return cmd.Help()
		}

		fmt.Printf("Detected PR: %s\n", detectedURL)
		args = []string{detectedURL}
	}

	if !flagWatch && !flagDownload && !flagAnalyze && !flagAnalyzeOnFailure {
		fmt.Fprintln(os.Stderr, "Error: at least one action flag is required (--watch, --download, --analyze, or --analyze-on-failure)")
		fmt.Fprintln(os.Stderr)
		return cmd.Help()
	}

	if flagAnalyze && flagAnalyzeOnFailure {
		fmt.Fprintln(os.Stderr, "Error: --analyze and --analyze-on-failure cannot be used together")
		os.Exit(ExitConfigError)
		return nil
	}

	prowURL := args[0]

	// If background mode, fork and exit parent
	if flagBackground {
		return runInBackground(os.Args)
	}

	return executeWorkflow(prowURL, flagNotifyComplete)
}

// runInBackground forks the current process to run in background
func runInBackground(args []string) error {
	// Remove --background flag from args
	newArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--background" || args[i] == "-background" {
			continue
		}
		newArgs = append(newArgs, args[i])
	}

	// Add internal flag to indicate we're in background mode (for notifications)
	newArgs = append(newArgs, "--notify-on-complete")

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Fork the process
	procAttr := &syscall.ProcAttr{
		Dir:   ".",
		Env:   os.Environ(),
		Files: []uintptr{0, 1, 2}, // stdin, stdout, stderr
	}

	pid, err := syscall.ForkExec(execPath, newArgs, procAttr)
	if err != nil {
		return fmt.Errorf("failed to fork process: %w", err)
	}

	fmt.Printf("Started background process with PID %d\n", pid)
	return nil
}

// loadConfig loads the merged configuration (CLI > env > file > defaults) and
// validates the destination-conflict policy. In background mode a "prompt"
// policy is coerced to "new" (timestamped folder) so that a detached process
// can never block forever waiting for interactive input.
func loadConfig() (*config.Config, downloader.ConflictPolicy, error) {
	cfg, err := config.Load(&config.Config{
		Dest:        flagDest,
		AnalyzeCmd:  flagAnalyzeCmd,
		NtfyChannel: flagNtfyChannel,
		Interval:    flagInterval,
		OnConflict:  flagOnConflict,
	}, flagConfig)
	if err != nil {
		return nil, "", err
	}

	policy, err := downloader.ParseConflictPolicy(cfg.OnConflict)
	if err != nil {
		return nil, "", err
	}

	if flagNotifyComplete && policy == downloader.PolicyPrompt {
		policy = downloader.PolicyNew
	}
	return cfg, policy, nil
}

// finishMonitorWorkflow handles the aftermath of a multi-job monitor run:
// downloads/analysis for completed jobs (skipped when interrupted) and the
// final exit code — ExitInterrupted on Ctrl+C, ExitJobFailed when any job failed.
func finishMonitorWorkflow(completed []*monitorEntry, interrupted bool, cfg *config.Config, policy downloader.ConflictPolicy, sendNotification bool) error {
	if interrupted {
		if anyDownloadAction() {
			fmt.Println("Skipping download (interrupted).")
		}
		os.Exit(ExitInterrupted)
		return nil
	}
	if anyDownloadAction() && len(completed) > 0 {
		downloadMonitoredEntries(completed, cfg, policy, sendNotification, flagDownload, flagAnalyze, flagAnalyzeOnFailure)
	}
	if anyMonitoredFailed(completed) {
		os.Exit(ExitJobFailed)
	}
	return nil
}

// executeWorkflow runs the main workflow based on the action flags:
//   - --watch: watch running jobs until completion
//   - --download: unconditionally download test artifacts
//   - --analyze: download (if needed) and analyze artifacts
//   - --analyze-on-failure: like --analyze, but only for failed jobs
func executeWorkflow(prowURL string, sendNotification bool) error {
	// Step 0: Load configuration up front; it doesn't depend on the URL and
	// every code path below needs it.
	cfg, policy, err := loadConfig()
	if err != nil {
		errMsg := fmt.Sprintf("Failed to load configuration: %v", err)
		fmt.Fprintln(os.Stderr, errMsg)
		if sendNotification {
			notifier.Notify("Configuration", errMsg, false)
		}
		os.Exit(ExitConfigError)
		return nil
	}

	if (flagAnalyze || flagAnalyzeOnFailure) && cfg.AnalyzeCmd == "" {
		errMsg := "Error: --analyze/--analyze-on-failure requires an analyze command (--analyze-cmd, PROW_HELPER_ANALYZE_CMD, or analyze_cmd in the config file)"
		fmt.Fprintln(os.Stderr, errMsg)
		os.Exit(ExitConfigError)
		return nil
	}

	// wantDownload is true whenever any action flag implies fetching artifacts.
	// --analyze-on-failure only implies it conditionally, once the job's
	// pass/fail status is known, so it's handled separately below.
	wantDownload := anyDownloadAction()

	// Step 1: Validate URL; if not a direct prow URL, try to resolve it
	if err := parser.ValidateURL(prowURL); err != nil {
		// Check if it's a GitHub PR URL first
		if pr := resolver.ParseGitHubPRURL(prowURL); pr != nil {
			fmt.Fprintf(os.Stdout, "GitHub PR detected: %s/%s #%d — fetching prow jobs...\n", pr.Org, pr.Repo, pr.Number)
			jobs, resolveErr := prowapi.FetchJobsForPR(prowHost, pr.Org, pr.Repo, pr.Number)
			if resolveErr != nil || len(jobs) == 0 {
				errMsg := fmt.Sprintf("Could not find prow jobs for PR: %v", resolveErr)
				if resolveErr == nil {
					errMsg = fmt.Sprintf("no prow jobs found for %s/%s#%d", pr.Org, pr.Repo, pr.Number)
				}
				fmt.Fprintln(os.Stderr, errMsg)
				if sendNotification {
					notifier.Notify("URL Validation", errMsg, false)
				}
				os.Exit(ExitInvalidURL)
				return nil
			}

			// Multiple jobs with --watch: use the multi-job monitor flow
			if flagWatch && len(jobs) > 1 {
				completed, interrupted, monErr := runMonitorFlowForPR(pr, jobs, cfg, policy)
				if monErr != nil {
					return monErr
				}
				return finishMonitorWorkflow(completed, interrupted, cfg, policy, sendNotification)
			}

			// Single job: select automatically
			if len(jobs) == 1 {
				fmt.Printf("Found prow job: %s (%s)\n", jobs[0].Name, jobs[0].State)
				prowURL = jobs[0].URL
			} else {
				// Multiple jobs without --watch: let user pick one
				resolved, pickErr := promptJobSelection(jobs, pr)
				if pickErr != nil {
					fmt.Fprintf(os.Stderr, "Job selection failed: %v\n", pickErr)
					os.Exit(ExitInvalidURL)
					return nil
				}
				prowURL = resolved
			}
		} else {
			// Not a PROW job URL and not a GitHub PR.
			// If --watch is set, try the monitor flow (status page URLs).
			if flagWatch {
				jobs, fetchErr := prowapi.FetchJobs(prowURL)
				if fetchErr == nil && len(jobs) > 0 {
					completed, interrupted, monErr := runMonitorFlow(prowURL, jobs, cfg, policy)
					if monErr != nil {
						return monErr
					}
					return finishMonitorWorkflow(completed, interrupted, cfg, policy, sendNotification)
				}
			}

			// Fall back to web page scanning
			fmt.Fprintf(os.Stdout, "Not a direct prow URL (%v), attempting to find prow job link on page...\n", err)
			resolved, resolveErr := resolveProwURL(prowURL)
			if resolveErr != nil {
				errMsg := fmt.Sprintf("Invalid PROW URL and could not resolve prow job link: %v", resolveErr)
				fmt.Fprintln(os.Stderr, errMsg)
				if sendNotification {
					notifier.Notify("URL Validation", errMsg, false)
				}
				os.Exit(ExitInvalidURL)
				return nil
			}
			prowURL = resolved
		}
	}

	// Step 2: Parse URL to get metadata
	metadata, err := parser.ParseURL(prowURL)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to parse URL: %v", err)
		fmt.Fprintln(os.Stderr, errMsg)
		if sendNotification {
			notifier.Notify("URL Parsing", errMsg, false)
		}
		os.Exit(ExitInvalidURL)
		return nil
	}

	output.PrintField(os.Stdout, "Job", metadata.JobName)
	if metadata.PRRef != "" {
		output.PrintField(os.Stdout, "PR", metadata.PRRef)
	}
	output.PrintField(os.Stdout, "Build ID", metadata.BuildID)

	if cfg.NtfyChannel != "" {
		output.PrintField(os.Stdout, "Ntfy channel", cfg.NtfyChannel)
	}

	// jobDisplay combines the PR reference (when available) with the job name for
	// use in console output and notification titles/messages.
	jobDisplay := metadata.JobName
	if metadata.PRRef != "" {
		jobDisplay = metadata.PRRef + " " + metadata.JobName
	}

	// Step 3: If any action may download artifacts, decide the destination now —
	// before a potentially hours-long watch — so any conflict question is asked
	// while the user is still at the terminal. Deletion for "overwrite" is
	// deferred until the download actually starts (Step 5).
	var (
		destBase       string
		destResolution downloader.ConflictResolution
	)
	if wantDownload {
		destBase = downloader.BuildDestinationPath(cfg.Dest, metadata)
		exists, checkErr := downloader.CheckDestinationConflict(destBase)
		if checkErr != nil {
			errMsg := fmt.Sprintf("Failed to resolve destination: %v", checkErr)
			fmt.Fprintln(os.Stderr, errMsg)
			sendNotificationWithConfig(jobDisplay, errMsg, false, cfg.NtfyChannel, sendNotification)
			os.Exit(ExitDownloadFailed)
			return nil
		}
		destResolution = downloader.Overwrite // no conflict: plain download into destBase
		if exists {
			destResolution, err = downloader.ResolveConflictAction(destBase, policy, os.Stdin, os.Stdout)
			if err != nil {
				errMsg := fmt.Sprintf("Failed to resolve destination: %v", err)
				fmt.Fprintln(os.Stderr, errMsg)
				sendNotificationWithConfig(jobDisplay, errMsg, false, cfg.NtfyChannel, sendNotification)
				os.Exit(ExitDownloadFailed)
				return nil
			}
		}
	}

	// Step 4: Determine the job's pass/fail status, either by watching it to
	// completion or, if --analyze-on-failure needs it and we're not
	// watching, with a single status check.
	var jobStatus *watcher.JobStatus
	if flagWatch {
		status, err := watcher.Watch(metadata, cfg.Interval, os.Stdout)
		if err != nil {
			errMsg := fmt.Sprintf("Watch failed: %v", err)
			fmt.Fprintln(os.Stderr, errMsg)
			sendNotificationWithConfig(jobDisplay, errMsg, false, cfg.NtfyChannel, true)
			os.Exit(ExitWatchFailed)
			return nil
		}
		jobStatus = status

		msg := output.FormatJobStatusMessage(jobDisplay, status.Passed)
		fmt.Println(msg)

		if !wantDownload {
			// Watch-only mode: notify and exit
			sendNotificationWithConfig(jobDisplay, notifier.FormatJobStatusMessage(jobDisplay, status.Passed), status.Passed, cfg.NtfyChannel, true)
			if !status.Passed {
				os.Exit(ExitJobFailed)
			}
			return nil
		}
	} else if flagAnalyzeOnFailure {
		finishedURL := watcher.BuildFinishedJSONURL(metadata)
		status, statusErr := watcher.CheckJobStatus(finishedURL)
		if statusErr != nil {
			errMsg := fmt.Sprintf("Failed to check job status: %v", statusErr)
			fmt.Fprintln(os.Stderr, errMsg)
			sendNotificationWithConfig(jobDisplay, errMsg, false, cfg.NtfyChannel, sendNotification)
			os.Exit(ExitWatchFailed)
			return nil
		}
		jobStatus = status
		if status == nil {
			fmt.Println("Warning: job has not finished yet; cannot verify pass/fail status for --analyze-on-failure")
		}
	}

	if !wantDownload {
		return nil
	}

	// doAnalyze/doDownload fold in the conditional --analyze-on-failure
	// behavior now that jobStatus (if relevant) is known.
	jobFailed := jobStatus != nil && !jobStatus.Passed
	doAnalyze := flagAnalyze || (flagAnalyzeOnFailure && jobFailed)
	doDownload := flagDownload || doAnalyze

	if !doDownload {
		if jobStatus == nil {
			fmt.Println("Job status unknown; skipping download and analysis (--analyze-on-failure)")
		} else {
			fmt.Println("Job passed; skipping download and analysis (--analyze-on-failure)")
			sendNotificationWithConfig(jobDisplay, notifier.FormatJobStatusMessage(jobDisplay, jobStatus.Passed), jobStatus.Passed, cfg.NtfyChannel, sendNotification || flagWatch)
		}
		return nil
	}

	// Step 5: Apply the destination resolution decided in Step 3
	destPath, skip, err := downloader.ApplyResolution(destBase, destResolution)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to resolve destination: %v", err)
		fmt.Fprintln(os.Stderr, errMsg)
		sendNotificationWithConfig(jobDisplay, errMsg, false, cfg.NtfyChannel, sendNotification)
		os.Exit(ExitDownloadFailed)
		return nil
	}

	if skip {
		fmt.Println("Skipping download, using existing artifacts")
	} else {
		// Step 6: Download artifacts
		output.PrintField(os.Stdout, "Downloading to", destPath)

		// Notify download start
		if sendNotification || cfg.NtfyChannel != "" {
			sendNotificationWithConfig(jobDisplay, notifier.FormatDownloadStartMessage(jobDisplay), true, cfg.NtfyChannel, sendNotification)
		}

		gcsPath := "gs://" + metadata.Bucket + "/" + metadata.Path
		if err := downloader.Download(gcsPath, destPath, os.Stdout, os.Stderr); err != nil {
			errMsg := fmt.Sprintf("Download failed: %v", err)
			fmt.Fprintln(os.Stderr, errMsg)
			sendNotificationWithConfig(jobDisplay, notifier.FormatFailureMessage(jobDisplay, err), false, cfg.NtfyChannel, sendNotification)
			os.Exit(ExitDownloadFailed)
			return nil
		}

		fmt.Println("Download complete!")

		// Notify download complete (only if we will run analysis)
		if (sendNotification || cfg.NtfyChannel != "") && doAnalyze {
			sendNotificationWithConfig(jobDisplay, notifier.FormatDownloadCompleteMessage(jobDisplay, destPath), true, cfg.NtfyChannel, sendNotification)
		}
	}

	// Step 7: Run analysis if requested
	if doAnalyze {
		output.PrintField(os.Stdout, "Running analysis", cfg.AnalyzeCmd+" "+destPath)

		// Notify analysis start
		if sendNotification || cfg.NtfyChannel != "" {
			sendNotificationWithConfig(jobDisplay, notifier.FormatAnalysisStartMessage(jobDisplay, cfg.AnalyzeCmd), true, cfg.NtfyChannel, sendNotification)
		}

		if err := analyzer.RunAnalysis(cfg.AnalyzeCmd, destPath); err != nil {
			errMsg := fmt.Sprintf("Analysis failed: %v", err)
			fmt.Fprintln(os.Stderr, errMsg)
			sendNotificationWithConfig(jobDisplay, notifier.FormatFailureMessage(jobDisplay, err), false, cfg.NtfyChannel, sendNotification)
			os.Exit(ExitAnalysisFailed)
			return nil
		}

		fmt.Println("Analysis complete!")

		sendNotificationWithConfig(jobDisplay, notifier.FormatAnalysisSuccessMessage(jobDisplay, destPath), true, cfg.NtfyChannel, sendNotification)
	} else {
		sendNotificationWithConfig(jobDisplay, notifier.FormatDownloadOnlyMessage(jobDisplay, destPath), true, cfg.NtfyChannel, sendNotification)
	}

	return nil
}

// downloadMonitoredEntries downloads artifacts for each completed job from
// the monitor flow, and analyzes them if requested.
//   - alwaysDownload (--download) unconditionally downloads every entry.
//   - analyzeAlways (--analyze) downloads and analyzes every entry.
//   - analyzeOnFailure (--analyze-on-failure) downloads and analyzes only
//     entries whose job failed.
//
// Destinations that were resolved upfront (before monitoring) are reused so
// no interactive question is asked at this point; entries without an upfront
// resolution fall back to the configured conflict policy.
func downloadMonitoredEntries(entries []*monitorEntry, cfg *config.Config, policy downloader.ConflictPolicy, sendNotification bool, alwaysDownload, analyzeAlways, analyzeOnFailure bool) {
	for _, e := range entries {
		if e.status == nil || !e.status.Finished {
			continue
		}

		jobDisplay := e.metadata.JobName
		if e.prRef != "" {
			jobDisplay = e.prRef + " " + e.metadata.JobName
		}

		jobFailed := !e.status.Passed
		doAnalyze := analyzeAlways || (analyzeOnFailure && jobFailed)
		doDownload := alwaysDownload || doAnalyze

		if !doDownload {
			fmt.Printf("Skipping %s: job passed (--analyze-on-failure)\n", jobDisplay)
			continue
		}

		var (
			destPath string
			skip     bool
			err      error
		)
		if e.destResolved {
			destPath, skip, err = downloader.ApplyResolution(e.destBase, e.resolution)
		} else {
			destPath, skip, err = downloader.ResolveDestinationWithPolicy(cfg.Dest, e.metadata, policy, os.Stdin, os.Stdout)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to resolve destination for %s: %v\n", jobDisplay, err)
			continue
		}

		if skip {
			fmt.Printf("Skipping download for %s, using existing artifacts\n", jobDisplay)
		} else {
			output.PrintField(os.Stdout, "Downloading", jobDisplay)
			output.PrintField(os.Stdout, "Downloading to", destPath)

			gcsPath := "gs://" + e.metadata.Bucket + "/" + e.metadata.Path
			if err := downloader.Download(gcsPath, destPath, os.Stdout, os.Stderr); err != nil {
				fmt.Fprintf(os.Stderr, "Download failed for %s: %v\n", jobDisplay, err)
				sendNotificationWithConfig(jobDisplay, notifier.FormatFailureMessage(jobDisplay, err), false, cfg.NtfyChannel, sendNotification)
				continue
			}

			fmt.Printf("Download complete: %s\n", jobDisplay)
		}

		if doAnalyze {
			output.PrintField(os.Stdout, "Running analysis", cfg.AnalyzeCmd+" "+destPath)
			if err := analyzer.RunAnalysis(cfg.AnalyzeCmd, destPath); err != nil {
				fmt.Fprintf(os.Stderr, "Analysis failed for %s: %v\n", jobDisplay, err)
				sendNotificationWithConfig(jobDisplay, notifier.FormatFailureMessage(jobDisplay, err), false, cfg.NtfyChannel, sendNotification)
				continue
			}
			fmt.Printf("Analysis complete: %s\n", jobDisplay)
			sendNotificationWithConfig(jobDisplay, notifier.FormatAnalysisSuccessMessage(jobDisplay, destPath), true, cfg.NtfyChannel, sendNotification)
		} else {
			sendNotificationWithConfig(jobDisplay, notifier.FormatDownloadOnlyMessage(jobDisplay, destPath), true, cfg.NtfyChannel, sendNotification)
		}
	}
}

// prowHost is the default Prow instance used when resolving GitHub PR URLs.
const prowHost = "prow.ci.openshift.org"

// promptJobSelection lets the user pick one prow job using the same fuzzy
// selector UI as the multi-job monitor flow. When the TUI cannot start (e.g.
// no TTY), it falls back to a plain numbered prompt so scripting still works.
func promptJobSelection(jobs []prowapi.Job, pr *resolver.GitHubPR) (string, error) {
	fmt.Printf("Found %d prow jobs for %s/%s#%d\n", len(jobs), pr.Org, pr.Repo, pr.Number)

	items := make([]selector.Item, len(jobs))
	for i, j := range jobs {
		items[i] = selector.Item{
			Key:   j.URL,
			Label: fmt.Sprintf("%-*s  %s", stateWidth, j.State, j.Name),
		}
	}

	idx, err := selector.RunSingle(items, nil)
	if err != nil {
		return promptJobSelectionNumbered(jobs)
	}
	if idx < 0 {
		return "", fmt.Errorf("selection cancelled")
	}
	return jobs[idx].URL, nil
}

// promptJobSelectionNumbered is the non-TUI fallback for promptJobSelection.
func promptJobSelectionNumbered(jobs []prowapi.Job) (string, error) {
	for i, j := range jobs {
		fmt.Printf("  [%d] %-12s %s\n", i+1, j.State, j.Name)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Select a job [1-%d]: ", len(jobs))
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read selection: %w", err)
		}
		n, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil || n < 1 || n > len(jobs) {
			fmt.Printf("Invalid selection, please enter a number between 1 and %d\n", len(jobs))
			continue
		}
		return jobs[n-1].URL, nil
	}
}

// runMonitorFlowForPR is like runMonitorFlow but refreshes jobs via FetchJobsForPR
// instead of FetchJobs, since GitHub PR URLs are not Prow status page URLs.
func runMonitorFlowForPR(pr *resolver.GitHubPR, jobs []prowapi.Job, cfg *config.Config, policy downloader.ConflictPolicy) ([]*monitorEntry, bool, error) {
	entries, items, err := buildEntriesAndItems(jobs)
	if err != nil {
		return nil, false, err
	}

	refreshFn := func() ([]selector.Item, error) {
		refreshed, fetchErr := prowapi.FetchJobsForPR(prowHost, pr.Org, pr.Repo, pr.Number)
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch prow jobs: %w", fetchErr)
		}
		if len(refreshed) == 0 {
			return nil, fmt.Errorf("no prow jobs found")
		}
		newEntries, newItems, buildErr := buildEntriesAndItems(refreshed)
		if buildErr != nil {
			return nil, buildErr
		}
		entries = newEntries
		return newItems, nil
	}

	selectedIndices, err := selector.Run(items, refreshFn)
	if err != nil {
		return nil, false, err
	}
	if len(selectedIndices) == 0 {
		fmt.Println("No jobs selected. Exiting.")
		return nil, false, nil
	}

	sort.Ints(selectedIndices)

	selected := make([]*monitorEntry, len(selectedIndices))
	for i, idx := range selectedIndices {
		selected[i] = entries[idx]
	}

	return monitorSelected(selected, cfg, policy)
}

// resolveProwURL fetches the given URL and extracts a prow job link from the page.
// If exactly one prow job link is found it is returned automatically.
// If multiple are found the user picks one using the fuzzy selector, falling
// back to a plain numbered prompt when the TUI cannot start (e.g. no TTY).
func resolveProwURL(pageURL string) (string, error) {
	links, err := resolver.FindProwJobLinks(pageURL)
	if err != nil {
		return "", err
	}

	if len(links) == 1 {
		fmt.Printf("Found prow job link: %s\n", links[0])
		return links[0], nil
	}

	// Multiple links found: let the user choose
	fmt.Printf("Found %d prow job links on page\n", len(links))

	items := make([]selector.Item, len(links))
	for i, link := range links {
		items[i] = selector.Item{Key: link, Label: link}
	}

	idx, err := selector.RunSingle(items, nil)
	if err != nil {
		return resolveProwURLNumbered(links)
	}
	if idx < 0 {
		return "", fmt.Errorf("selection cancelled")
	}
	return links[idx], nil
}

// resolveProwURLNumbered is the non-TUI fallback for resolveProwURL.
func resolveProwURLNumbered(links []string) (string, error) {
	for i, link := range links {
		fmt.Printf("  [%d] %s\n", i+1, link)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Select a link [1-%d]: ", len(links))
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("failed to read selection: %w", err)
		}
		n, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil || n < 1 || n > len(links) {
			fmt.Printf("Invalid selection, please enter a number between 1 and %d\n", len(links))
			continue
		}
		return links[n-1], nil
	}
}

// sendNotificationWithConfig sends notifications using configured methods.
// ntfy.sh is used whenever ntfyChannel is non-empty, regardless of background mode.
// Desktop notification is sent only when sendDesktop is true (background mode).
func sendNotificationWithConfig(title, message string, success bool, ntfyChannel string, sendDesktop bool) {
	statusIcon := "Success"
	if !success {
		statusIcon = "Failed"
	}
	fullTitle := fmt.Sprintf("prow-helper: %s - %s", title, statusIcon)

	if ntfyChannel != "" {
		if err := notifier.NotifyNtfy(ntfyChannel, fullTitle, message); err != nil {
			fmt.Printf("Warning: ntfy notification failed: %v\n", err)
		}
	}

	if sendDesktop {
		if err := notifier.Notify(title, message, success); err != nil {
			fmt.Printf("Warning: desktop notification failed: %v\n", err)
		}
	}
}

// For testing: allow overriding exec.Command
var execCommand = exec.Command
