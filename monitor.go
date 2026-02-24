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

	"github.com/clobrano/prow-helper/internal/notifier"
	"github.com/clobrano/prow-helper/internal/output"
	"github.com/clobrano/prow-helper/internal/parser"
	"github.com/clobrano/prow-helper/internal/prowapi"
	"github.com/clobrano/prow-helper/internal/selector"
	"github.com/clobrano/prow-helper/internal/watcher"
)

var flagMonitorInterval time.Duration
var flagMonitorNtfyChannel string

var monitorCmd = &cobra.Command{
	Use:   "monitor <prow-status-url>",
	Short: "Fetch and monitor prow jobs from a status page",
	Long: `monitor fetches all prow job links from a Prow status page (e.g. filtered by
author) and lets you choose which jobs to watch.

The Prow status page is a React SPA — job data is loaded at runtime from the
/prowjobs.js API. The monitor command calls that API directly and filters by
any query parameters present in the URL (author, job, state).

An interactive list lets you select which jobs to monitor:
  Type       – filter the list (substring match against job name / state)
  ↑ / ↓     – move the cursor
  SPACE      – toggle the job under the cursor
  Ctrl+A     – select / deselect all visible jobs
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
	monitorCmd.Flags().StringVar(&flagMonitorNtfyChannel, "ntfy-channel", "", "ntfy.sh channel for push notifications")
	rootCmd.AddCommand(monitorCmd)
}

// monitorEntry holds the parsed metadata for a prow job and its latest known status.
type monitorEntry struct {
	metadata       *parser.ProwMetadata
	state          string             // original state from the API (triggered, pending, success, …)
	startTime      time.Time          // zero if the API did not provide one
	completionTime time.Time          // zero while still running
	status         *watcher.JobStatus // nil while still running
	err            error
	notified       bool // true once a completion notification has been sent
}

// formatTimeSuffix returns " (sch: HH:MM, dur: Xm Xs)" when startTime is known.
// end should be the completion time for finished jobs, or zero for running ones
// (in which case the elapsed time up to now is used).
func formatTimeSuffix(start, end time.Time) string {
	if start.IsZero() {
		return ""
	}
	var dur time.Duration
	if !end.IsZero() {
		dur = end.Sub(start)
	} else {
		dur = time.Since(start)
	}
	return fmt.Sprintf(" (sch: %s, dur: %s)", start.Local().Format("Jan 02 15:04"), dur.Truncate(time.Second))
}

// stateWidth is the column width reserved for Prow state strings.
// "triggered" (9 chars) is the longest state word.
const stateWidth = 9

// buildEntriesAndItems converts a slice of API jobs into parallel slices of
// monitorEntry and selector.Item.  Items whose URL cannot be parsed are
// skipped with a warning.
func buildEntriesAndItems(jobs []prowapi.Job) ([]*monitorEntry, []selector.Item, error) {
	entries := make([]*monitorEntry, 0, len(jobs))
	keys := make([]string, 0, len(jobs))
	for _, j := range jobs {
		meta, parseErr := parser.ParseURL(j.URL)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not parse job URL %s: %v\n", j.URL, parseErr)
			continue
		}
		entries = append(entries, &monitorEntry{
			metadata:       meta,
			state:          j.State,
			startTime:      j.StartTime,
			completionTime: j.CompletionTime,
		})
		keys = append(keys, j.URL)
	}
	if len(entries) == 0 {
		return nil, nil, fmt.Errorf("no valid prow job URLs found")
	}
	idxWidth := len(fmt.Sprintf("%d", len(entries)))
	items := make([]selector.Item, len(entries))
	for i, e := range entries {
		items[i] = selector.Item{
			Key: keys[i],
			Label: fmt.Sprintf("[%*d] %-*s  %s%s",
				idxWidth, i+1,
				stateWidth, e.state,
				e.metadata.JobName,
				formatTimeSuffix(e.startTime, e.completionTime)),
		}
	}
	return entries, items, nil
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

	entries, items, err := buildEntriesAndItems(jobs)
	if err != nil {
		return err
	}

	refreshFn := func() ([]selector.Item, error) {
		refreshed, fetchErr := prowapi.FetchJobs(pageURL)
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
	return monitorJobs(selected, flagMonitorInterval, flagMonitorNtfyChannel)
}

// monitorJobs polls all selected jobs until they all complete, printing a
// status table after each check round.
func monitorJobs(entries []*monitorEntry, interval time.Duration, ntfyChannel string) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Initial check immediately so we don't wait a full interval before first output.
	checkAllStatuses(entries)
	notifyCompletions(entries, ntfyChannel)
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
			notifyCompletions(entries, ntfyChannel)
			printStatusTable(entries)
		}
	}
}

// notifyCompletions sends a desktop and/or ntfy notification for each entry
// that just transitioned to a finished state and has not yet been notified.
func notifyCompletions(entries []*monitorEntry, ntfyChannel string) {
	for _, e := range entries {
		if e.notified {
			continue
		}
		if e.status == nil || !e.status.Finished {
			continue
		}
		e.notified = true
		msg := notifier.FormatJobStatusMessage(e.metadata.JobName, e.status.Passed)
		sendNotificationWithConfig(e.metadata.JobName, msg, e.status.Passed, ntfyChannel, true)
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
		// For running jobs use live elapsed time; for finished use the watcher timestamp.
		var endTime time.Time
		if e.status != nil && e.status.Finished {
			endTime = e.status.Timestamp
		}
		fmt.Printf("  [%*d] %-*s  %s%s\n",
			idxWidth, i+1,
			stateWidth, statusStr,
			e.metadata.JobName,
			formatTimeSuffix(e.startTime, endTime))
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
