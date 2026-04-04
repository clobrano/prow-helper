package main

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/clobrano/prow-helper/internal/notifier"
	"github.com/clobrano/prow-helper/internal/output"
	"github.com/clobrano/prow-helper/internal/parser"
	"github.com/clobrano/prow-helper/internal/prowapi"
	"github.com/clobrano/prow-helper/internal/selector"
	"github.com/clobrano/prow-helper/internal/watcher"
)

// monitorEntry holds the parsed metadata for a prow job and its latest known status.
type monitorEntry struct {
	metadata       *parser.ProwMetadata
	prRef          string             // "[org/repo PR<num>]" or "" for non-PR jobs
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
			prRef:          j.PRRef,
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
		jobDisplay := e.metadata.JobName
		if e.prRef != "" {
			jobDisplay = e.prRef + " " + e.metadata.JobName
		}
		items[i] = selector.Item{
			Key: keys[i],
			Label: fmt.Sprintf("[%*d] %-*s  %s%s",
				idxWidth, i+1,
				stateWidth, e.state,
				jobDisplay,
				formatTimeSuffix(e.startTime, e.completionTime)),
		}
	}
	return entries, items, nil
}

// runMonitorFlow presents an interactive job selector and monitors the selected
// jobs until they all complete. It is called from executeWorkflow when --watch
// is used with a Prow status page URL.
// Returns the monitored entries after completion (nil if no jobs were selected).
func runMonitorFlow(pageURL string, jobs []prowapi.Job, interval time.Duration, ntfyChannel string) ([]*monitorEntry, error) {
	entries, items, err := buildEntriesAndItems(jobs)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if len(selectedIndices) == 0 {
		fmt.Println("No jobs selected. Exiting.")
		return nil, nil
	}

	// Restore original order (selector returns indices in map-iteration order).
	sort.Ints(selectedIndices)

	selected := make([]*monitorEntry, len(selectedIndices))
	for i, idx := range selectedIndices {
		selected[i] = entries[idx]
	}

	fmt.Fprintf(os.Stdout, "\nMonitoring %d job(s) (interval: %s)...\n\n", len(selected), interval)
	if err := monitorJobs(selected, interval, ntfyChannel); err != nil {
		return nil, err
	}
	return selected, nil
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
	// Silence notifications for jobs that were already finished before we started.
	markAlreadyFinished(entries)
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

// markAlreadyFinished silently sets notified=true for every entry that is
// already in a terminal state so that we never fire a notification for jobs
// that were complete before monitoring began.
func markAlreadyFinished(entries []*monitorEntry) {
	for _, e := range entries {
		if e.status != nil && e.status.Finished {
			e.notified = true
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
		jobDisplay := e.metadata.JobName
		if e.prRef != "" {
			jobDisplay = e.prRef + " " + jobDisplay
		}
		msg := notifier.FormatJobStatusMessage(jobDisplay, e.status.Passed)
		sendNotificationWithConfig(jobDisplay, msg, e.status.Passed, ntfyChannel, true)
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
		jobDisplay := e.metadata.JobName
		if e.prRef != "" {
			jobDisplay = e.prRef + " " + e.metadata.JobName
		}
		fmt.Printf("  [%*d] %-*s  %s%s\n",
			idxWidth, i+1,
			stateWidth, statusStr,
			jobDisplay,
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
