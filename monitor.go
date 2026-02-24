package main

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/clobrano/prow-helper/internal/output"
	"github.com/clobrano/prow-helper/internal/parser"
	"github.com/clobrano/prow-helper/internal/prowapi"
	"github.com/clobrano/prow-helper/internal/selector"
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

An interactive fuzzy-search list lets you select which jobs to monitor:
  Type       – filter the list (fuzzy match against job name / state / build)
  ↑ / ↓     – move the cursor
  SPACE      – toggle the job under the cursor
  A          – select / deselect all visible jobs
  ENTER      – confirm the selection and start monitoring
  ESC        – clear the search (first press) or cancel (second press)

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
	state    string             // original state from the API (triggered, pending, success, …)
	status   *watcher.JobStatus // nil while still running
	err      error
}

// stateWidth is the column width reserved for Prow state strings.
// "triggered" (9 chars) is the longest state word.
const stateWidth = 9

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

	// Build selector items — one label per entry formatted as the status table.
	idxWidth := len(fmt.Sprintf("%d", len(entries)))
	items := make([]selector.Item, len(entries))
	for i, e := range entries {
		items[i] = selector.Item{
			Label: fmt.Sprintf("[%*d] %-*s  %s  %s",
				idxWidth, i+1,
				stateWidth, e.state,
				e.metadata.JobName,
				e.metadata.BuildID),
		}
	}

	selectedIndices, err := selector.Run(items)
	if err != nil {
		return err
	}
	if len(selectedIndices) == 0 {
		fmt.Println("No jobs selected. Exiting.")
		return nil
	}

	// Restore original order (selector returns indices in map-iteration order).
	sort.Ints(selectedIndices)

	selected := make([]*monitorEntry, len(selectedIndices))
	for i, idx := range selectedIndices {
		selected[i] = entries[idx]
	}

	fmt.Fprintf(os.Stdout, "\nMonitoring %d job(s) (interval: %s)...\n\n", len(selected), flagMonitorInterval)
	return monitorJobs(selected, flagMonitorInterval)
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
	idxWidth := len(fmt.Sprintf("%d", len(entries)))
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
		fmt.Printf("  [%*d] %-*s  %s  %s\n",
			idxWidth, i+1,
			stateWidth, statusStr,
			e.metadata.JobName,
			e.metadata.BuildID)
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
