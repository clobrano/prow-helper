package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/clobrano/prow-helper/internal/analyzer"
	"github.com/clobrano/prow-helper/internal/config"
	"github.com/clobrano/prow-helper/internal/downloader"
	"github.com/clobrano/prow-helper/internal/notifier"
	"github.com/clobrano/prow-helper/internal/output"
	"github.com/clobrano/prow-helper/internal/parser"
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
)

var (
	// CLI flags
	flagDest           string
	flagAnalyzeCmd     string
	flagBackground     bool
	flagNotifyComplete bool // Internal flag set by background mode
	flagWatch          bool
	flagNtfyChannel    string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "prow-helper <prow-url>",
	Short: "Download and analyze PROW CI test artifacts",
	Long: `prow-helper automates the workflow of downloading PROW CI test artifacts
and running analysis on them.

It takes a PROW test URL (e.g., https://prow.ci.openshift.org/view/gs/...),
downloads the artifacts using gsutil, and optionally runs a configured
analysis command on the downloaded files.

Example:
  prow-helper https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-name/12345

  prow-helper --dest ~/artifacts --analyze-cmd "my-analyzer" <url>

  prow-helper --background <url>

  prow-helper --watch <url>

  prow-helper --watch --ntfy-channel my-channel <url>`,
	Args: cobra.ExactArgs(1),
	RunE: runMain,
}

func init() {
	rootCmd.Flags().StringVar(&flagDest, "dest", "", "Download destination directory")
	rootCmd.Flags().StringVar(&flagAnalyzeCmd, "analyze-cmd", "", "Command to run after download")
	rootCmd.Flags().BoolVar(&flagBackground, "background", false, "Run in background and notify when done")
	rootCmd.Flags().BoolVar(&flagNotifyComplete, "notify-on-complete", false, "Internal flag for background mode notifications")
	rootCmd.Flags().MarkHidden("notify-on-complete") // Hide from help output
	rootCmd.Flags().BoolVar(&flagWatch, "watch", false, "Poll job status until completion before downloading")
	rootCmd.Flags().StringVar(&flagNtfyChannel, "ntfy-channel", "", "ntfy.sh channel for notifications")
	rootCmd.Version = Version
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runMain(cmd *cobra.Command, args []string) error {
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

// executeWorkflow runs the main download and analysis workflow
func executeWorkflow(prowURL string, sendNotification bool) error {

	// Step 1: Validate URL
	if err := parser.ValidateURL(prowURL); err != nil {
		errMsg := fmt.Sprintf("Invalid PROW URL: %v\nExpected format: https://prow.ci.openshift.org/view/gs/<bucket>/<path>", err)
		fmt.Fprintln(os.Stderr, errMsg)
		if sendNotification {
			notifier.Notify("URL Validation", errMsg, false)
		}
		os.Exit(ExitInvalidURL)
		return nil
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
	output.PrintField(os.Stdout, "Build ID", metadata.BuildID)

	// Step 3: Load configuration
	cliConfig := &config.Config{
		Dest:        flagDest,
		AnalyzeCmd:  flagAnalyzeCmd,
		NtfyChannel: flagNtfyChannel,
	}

	cfg, err := config.Load(cliConfig)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to load configuration: %v", err)
		fmt.Fprintln(os.Stderr, errMsg)
		if sendNotification {
			notifier.Notify("Configuration", errMsg, false)
		}
		os.Exit(ExitConfigError)
		return nil
	}

	if cfg.NtfyChannel != "" {
		output.PrintField(os.Stdout, "Ntfy channel", cfg.NtfyChannel)
	}

	// Step 4: If watch mode, poll until job completes
	if flagWatch {
		status, err := watcher.Watch(metadata, watcher.DefaultPollInterval, os.Stdout)
		if err != nil {
			errMsg := fmt.Sprintf("Watch failed: %v", err)
			fmt.Fprintln(os.Stderr, errMsg)
			sendNotificationWithConfig(metadata.JobName, errMsg, false, cfg.NtfyChannel, true)
			os.Exit(ExitWatchFailed)
			return nil
		}

		if !status.Passed {
			// Job failed
			msg := output.FormatJobStatusMessage(metadata.JobName, false)
			fmt.Println(msg)

			// If no analyze command, just notify and exit
			if cfg.AnalyzeCmd == "" {
				sendNotificationWithConfig(metadata.JobName, notifier.FormatJobStatusMessage(metadata.JobName, false), false, cfg.NtfyChannel, true)
				os.Exit(ExitJobFailed)
				return nil
			}
			// If analyze command is set, continue to download artifacts for analysis
		} else {
			// Job passed
			msg := output.FormatJobStatusMessage(metadata.JobName, true)
			fmt.Println(msg)

			// If no analyze command, just notify and exit
			if cfg.AnalyzeCmd == "" {
				sendNotificationWithConfig(metadata.JobName, notifier.FormatJobStatusMessage(metadata.JobName, true), true, cfg.NtfyChannel, true)
				return nil
			}
			// If analyze command is set, continue to download artifacts for analysis
		}
	}

	// Step 5: Resolve destination with conflict handling
	destPath, skip, err := downloader.ResolveDestination(cfg.Dest, metadata, os.Stdin, os.Stdout)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to resolve destination: %v", err)
		fmt.Fprintln(os.Stderr, errMsg)
		sendNotificationWithConfig("Destination", errMsg, false, cfg.NtfyChannel, sendNotification)
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
			sendNotificationWithConfig(metadata.JobName, notifier.FormatDownloadStartMessage(metadata.JobName), true, cfg.NtfyChannel, sendNotification)
		}

		gcsPath := "gs://" + metadata.Bucket + "/" + metadata.Path
		if err := downloader.Download(gcsPath, destPath, os.Stdout, os.Stderr); err != nil {
			errMsg := fmt.Sprintf("Download failed: %v", err)
			fmt.Fprintln(os.Stderr, errMsg)
			sendNotificationWithConfig(metadata.JobName, notifier.FormatFailureMessage(metadata.JobName, err), false, cfg.NtfyChannel, sendNotification)
			os.Exit(ExitDownloadFailed)
			return nil
		}

		fmt.Println("Download complete!")

		// Step 5.5: Rename folder with date prefix from started.json
		newDestPath, err := downloader.RenameWithDatePrefix(destPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to rename folder with date prefix: %v\n", err)
			fmt.Fprintln(os.Stderr, "Continuing with original folder name...")
		} else {
			fmt.Printf("Renamed folder to: %s\n", newDestPath)
			destPath = newDestPath // Update destPath for analysis
		}

		// Notify download complete (only if we will run analysis)
		if (sendNotification || cfg.NtfyChannel != "") && cfg.AnalyzeCmd != "" {
			sendNotificationWithConfig(metadata.JobName, notifier.FormatDownloadCompleteMessage(metadata.JobName, destPath), true, cfg.NtfyChannel, sendNotification)
		}
	}

	// Step 7: Run analysis command if configured
	if cfg.AnalyzeCmd != "" {
		output.PrintField(os.Stdout, "Running analysis", cfg.AnalyzeCmd+" "+destPath)

		// Notify analysis start
		if sendNotification || cfg.NtfyChannel != "" {
			sendNotificationWithConfig(metadata.JobName, notifier.FormatAnalysisStartMessage(metadata.JobName, cfg.AnalyzeCmd), true, cfg.NtfyChannel, sendNotification)
		}

		if err := analyzer.RunAnalysis(cfg.AnalyzeCmd, destPath); err != nil {
			var exitErr *analyzer.ExitError
			if errors.As(err, &exitErr) {
				errMsg := fmt.Sprintf("Analysis failed with exit code %d", exitErr.ExitCode)
				fmt.Fprintln(os.Stderr, errMsg)
				sendNotificationWithConfig(metadata.JobName, notifier.FormatFailureMessage(metadata.JobName, err), false, cfg.NtfyChannel, sendNotification)
				os.Exit(ExitAnalysisFailed)
				return nil
			}

			errMsg := fmt.Sprintf("Analysis failed: %v", err)
			fmt.Fprintln(os.Stderr, errMsg)
			sendNotificationWithConfig(metadata.JobName, notifier.FormatFailureMessage(metadata.JobName, err), false, cfg.NtfyChannel, sendNotification)
			os.Exit(ExitAnalysisFailed)
			return nil
		}

		fmt.Println("Analysis complete!")

		sendNotificationWithConfig(metadata.JobName, notifier.FormatAnalysisSuccessMessage(metadata.JobName, destPath), true, cfg.NtfyChannel, sendNotification)
	} else {
		sendNotificationWithConfig(metadata.JobName, notifier.FormatDownloadOnlyMessage(metadata.JobName, destPath), true, cfg.NtfyChannel, sendNotification)
	}

	return nil
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
		notifier.Notify(title, message, success)
	}
}

// For testing: allow overriding exec.Command
var execCommand = exec.Command
