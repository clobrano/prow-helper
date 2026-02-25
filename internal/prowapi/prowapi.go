// Package prowapi fetches prow job data from the Prow Deck /prowjobs.js endpoint.
//
// The Prow status page (e.g. https://prow.ci.openshift.org/?author=clobrano) is a
// React SPA whose job list is rendered client-side from the /prowjobs.js API.  A
// plain HTTP GET of the status page returns only a script-tag shell, so this
// package calls the underlying JSON API directly.
package prowapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Job holds the fields of a ProwJob that are relevant for monitoring.
type Job struct {
	Name           string
	State          string
	URL            string
	Author         string
	PRRef          string    // "[org/repo PR<num>]" for presubmit jobs, "" otherwise
	StartTime      time.Time // zero if not yet started
	CompletionTime time.Time // zero if still running
}

// prowJobList is the top-level structure returned by /prowjobs.js.
type prowJobList struct {
	Items []prowJob `json:"items"`
}

type prowJob struct {
	Spec   prowJobSpec   `json:"spec"`
	Status prowJobStatus `json:"status"`
}

type prowJobSpec struct {
	Job  string       `json:"job"`
	Type string       `json:"type"`
	Refs *prowJobRefs `json:"refs"`
}

type prowJobRefs struct {
	Org   string        `json:"org"`
	Repo  string        `json:"repo"`
	Pulls []prowJobPull `json:"pulls"`
}

type prowJobPull struct {
	Author string `json:"author"`
	Number int    `json:"number"`
}

type prowJobStatus struct {
	State          string `json:"state"`
	URL            string `json:"url"`
	StartTime      string `json:"startTime"`
	CompletionTime string `json:"completionTime"`
}

// FetchJobs calls <host>/prowjobs.js and returns the jobs that match the
// filter query parameters found in pageURL (author, job, state).
//
// The /prowjobs.js endpoint returns a JavaScript assignment of the form
//
//	var allBuilds = <ProwJobList JSON>
//
// which is stripped to obtain the underlying JSON before parsing.
func FetchJobs(pageURL string) ([]Job, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	apiURL := &url.URL{
		Scheme:   u.Scheme,
		Host:     u.Host,
		Path:     "/prowjobs.js",
		RawQuery: "omit=annotations,labels,decoration_config,pod_spec",
	}

	resp, err := http.Get(apiURL.String()) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("failed to fetch prowjobs.js: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prowjobs.js returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	jobs, err := parse(body)
	if err != nil {
		return nil, err
	}

	return filter(jobs, u.Query()), nil
}

// parse strips the JavaScript variable prefix and decodes the ProwJobList JSON.
func parse(body []byte) ([]Job, error) {
	data := strings.TrimSpace(string(body))

	// Strip "var <name> = " prefix — find the first '{'.
	if idx := strings.Index(data, "{"); idx > 0 {
		data = data[idx:]
	}
	// Strip trailing semicolon.
	data = strings.TrimRight(data, "; \t\n\r")

	var list prowJobList
	if err := json.Unmarshal([]byte(data), &list); err != nil {
		return nil, fmt.Errorf("failed to parse prowjobs.js JSON: %w", err)
	}

	jobs := make([]Job, 0, len(list.Items))
	for _, pj := range list.Items {
		if pj.Status.URL == "" {
			continue
		}
		j := Job{
			Name:  pj.Spec.Job,
			State: pj.Status.State,
			URL:   pj.Status.URL,
		}
		if pj.Spec.Refs != nil && len(pj.Spec.Refs.Pulls) > 0 {
			j.Author = pj.Spec.Refs.Pulls[0].Author
			if pj.Spec.Refs.Org != "" && pj.Spec.Refs.Repo != "" && pj.Spec.Refs.Pulls[0].Number > 0 {
				j.PRRef = fmt.Sprintf("[%s/%s PR%d]", pj.Spec.Refs.Org, pj.Spec.Refs.Repo, pj.Spec.Refs.Pulls[0].Number)
			}
		}
		if pj.Status.StartTime != "" {
			j.StartTime, _ = time.Parse(time.RFC3339, pj.Status.StartTime)
		}
		if pj.Status.CompletionTime != "" {
			j.CompletionTime, _ = time.Parse(time.RFC3339, pj.Status.CompletionTime)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// filter applies query-parameter-based filters to a job list.
// Recognised parameters: author, job (substring match), state.
func filter(jobs []Job, q url.Values) []Job {
	authorFilter := q.Get("author")
	stateFilter := q.Get("state")
	jobFilter := q.Get("job")

	if authorFilter == "" && stateFilter == "" && jobFilter == "" {
		return jobs
	}

	result := make([]Job, 0, len(jobs))
	for _, j := range jobs {
		if authorFilter != "" && j.Author != authorFilter {
			continue
		}
		if stateFilter != "" && j.State != stateFilter {
			continue
		}
		if jobFilter != "" && !strings.Contains(j.Name, jobFilter) {
			continue
		}
		result = append(result, j)
	}
	return result
}
