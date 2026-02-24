package main

import (
	"bufio"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/clobrano/prow-helper/internal/output"
	"github.com/clobrano/prow-helper/internal/parser"
	"github.com/clobrano/prow-helper/internal/prowapi"
	"github.com/clobrano/prow-helper/internal/watcher"
)

var flagMonitorInterval time.Duration

var monitorCmd = &cobra.Command{
	Use:   "monitor <prow-status-url>",
	Short: "Fetch and monitor prow jobs from a status page",
	Long: `monitor fetches all prow job links from a Prow status page (e.g. filtered by
author) and lets you choose which jobs to watch.

The Prow status page is a React SPA — job data is loaded at runtime from the
/prowjobs.js API. The monitor command calls that API directly and filters by
any query parameters present in the URL (author, job, state).

The tool presents the discovered jobs and prompts for a selection:
  all        – monitor every job found
  none       – exit without monitoring
  1,3,5      – monitor the jobs at those positions
  1-3        – monitor a range of jobs

Example:
  prow-helper monitor https://prow.ci.openshift.org/?author=clobrano`,
	Args: cobra.ExactArgs(1),
	RunE: runMonitor,
}

func init() {
	monitorCmd.Flags().DurationVar(&flagMonitorInterval, "interval", watcher.DefaultPollInterval,
		"Polling interval for job status checks")
	rootCmd.AddCommand(monitorCmd)
}

// monitorEntry holds the parsed metadata for a prow job and its latest known status.
type monitorEntry struct {
	metadata *parser.ProwMetadata
	state    string // original state from the API (triggered, pending, success, …)
	status   *watcher.JobStatus // nil while still running
	err      error
}

func runMonitor(cmd *cobra.Command, args []string) error {
	pageURL := args[0]

	fmt.Fprintf(os.Stdout, "Fetching prow jobs from %s...\n", pageURL)

	jobs, err := prowapi.FetchJobs(pageURL)
	if err != nil {
		return fmt.Errorf("failed to fetch prow jobs: %w", err)
	}

	if len(jobs) == 0 {
		return fmt.Errorf("no prow jobs found (try adjusting the filter parameters in the URL)")
	}

	// Parse each job URL into metadata; skip any we cannot parse.
	entries := make([]*monitorEntry, 0, len(jobs))
	for _, j := range jobs {
		meta, parseErr := parser.ParseURL(j.URL)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not parse job URL %s: %v\n", j.URL, parseErr)
			continue
		}
		entries = append(entries, &monitorEntry{
			metadata: meta,
			state:    j.State,
		})
	}

	if len(entries) == 0 {
		return fmt.Errorf("no valid prow job URLs found")
	}

	// Display the full list so the user can make an informed choice.
	fmt.Fprintf(os.Stdout, "\nFound %d prow job(s):\n\n", len(entries))
	printEntryList(entries)
	fmt.Println()

	selected, err := promptJobSelection(entries)
	if err != nil {
		return err
	}

	if len(selected) == 0 {
		fmt.Println("No jobs selected. Exiting.")
		return nil
	}

	fmt.Fprintf(os.Stdout, "\nMonitoring %d job(s) (interval: %s)...\n\n", len(selected), flagMonitorInterval)
	return monitorJobs(selected, flagMonitorInterval)
}

// printEntryList prints the indexed job list with name, build ID and current state.
func printEntryList(entries []*monitorEntry) {
	for i, e := range entries {
		fmt.Fprintf(os.Stdout, "  [%d] %-55s build: %-15s state: %s\n",
			i+1, e.metadata.JobName, e.metadata.BuildID, e.state)
	}
}

// promptJobSelection asks the user which jobs to monitor and returns the chosen subset.
func promptJobSelection(entries []*monitorEntry) ([]*monitorEntry, error) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Select jobs to monitor [all/none/1,2,3]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read selection: %w", err)
		}
		input = strings.TrimSpace(input)

		switch strings.ToLower(input) {
		case "all", "a":
			return entries, nil
		case "none", "n", "":
			return nil, nil
		default:
			selected, parseErr := parseSelection(input, entries)
			if parseErr != nil {
				fmt.Fprintf(os.Stderr, "Invalid selection: %v. Try again.\n", parseErr)
				continue
			}
			return selected, nil
		}
	}
}

// parseSelection converts a comma-separated (with optional ranges) string into
// a slice of entries. Examples: "1,3,5"  "2-4"  "1,3-5".
func parseSelection(input string, entries []*monitorEntry) ([]*monitorEntry, error) {
	seen := make(map[int]bool)
	var selected []*monitorEntry

	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			// Range notation: "lo-hi"
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			hi, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil || lo < 1 || hi > len(entries) || lo > hi {
				return nil, fmt.Errorf("invalid range %q (valid: 1-%d)", part, len(entries))
			}
			for i := lo; i <= hi; i++ {
				if !seen[i] {
					seen[i] = true
					selected = append(selected, entries[i-1])
				}
			}
		} else {
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 || n > len(entries) {
				return nil, fmt.Errorf("invalid index %q (valid: 1-%d)", part, len(entries))
			}
			if !seen[n] {
				seen[n] = true
				selected = append(selected, entries[n-1])
			}
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no valid indices found in %q", input)
	}
	return selected, nil
}

// monitorJobs polls all selected jobs until they all complete, printing a
// status table after each check round.
func monitorJobs(entries []*monitorEntry, interval time.Duration) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial check immediately so we don't wait a full interval before first output.
	checkAllStatuses(entries)
	printStatusTable(entries)

	for {
		if allEntriesDone(entries) {
			fmt.Println("\nAll monitored jobs have completed.")
			printFinalSummary(entries)
			return nil
		}

		select {
		case <-sigCh:
			fmt.Println("\nInterrupted.")
			return nil
		case <-ticker.C:
			checkAllStatuses(entries)
			printStatusTable(entries)
		}
	}
}

// checkAllStatuses fetches the current finished.json status for every entry
// that has not yet completed. Checks are performed concurrently.
func checkAllStatuses(entries []*monitorEntry) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, e := range entries {
		if e.status != nil && e.status.Finished {
			continue // already done
		}
		wg.Add(1)
		e := e
		go func() {
			defer wg.Done()
			finishedURL := watcher.BuildFinishedJSONURL(e.metadata)
			status, err := watcher.CheckJobStatus(finishedURL)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				e.err = err
			} else if status != nil {
				e.status = status
			}
		}()
	}
	wg.Wait()
}

// allEntriesDone returns true when every entry has a finished status or an error.
func allEntriesDone(entries []*monitorEntry) bool {
	for _, e := range entries {
		if e.err == nil && (e.status == nil || !e.status.Finished) {
			return false
		}
	}
	return true
}

// printStatusTable prints the current status of all monitored jobs.
func printStatusTable(entries []*monitorEntry) {
	fmt.Printf("[%s]\n", time.Now().Format("15:04:05"))
	for i, e := range entries {
		var statusStr string
		switch {
		case e.err != nil:
			statusStr = output.FormatStatus(output.StatusFailed) + fmt.Sprintf(" (error: %v)", e.err)
		case e.status == nil || !e.status.Finished:
			statusStr = output.FormatStatus(output.StatusRunning)
		case e.status.Passed:
			statusStr = output.FormatStatus(output.StatusSucceeded)
		default:
			statusStr = output.FormatStatus(output.StatusFailed)
		}
		fmt.Printf("  [%d] %-55s build: %-15s %s\n",
			i+1, e.metadata.JobName, e.metadata.BuildID, statusStr)
	}
	fmt.Println()
}

// printFinalSummary prints a summary of pass/fail counts once all jobs are done.
func printFinalSummary(entries []*monitorEntry) {
	fmt.Println("Summary:")
	passed, failed, errored := 0, 0, 0
	for _, e := range entries {
		switch {
		case e.err != nil:
			errored++
		case e.status != nil && e.status.Passed:
			passed++
		default:
			failed++
		}
	}
	fmt.Printf("  Passed:  %d\n", passed)
	fmt.Printf("  Failed:  %d\n", failed)
	if errored > 0 {
		fmt.Printf("  Errored: %d\n", errored)
	}
}
