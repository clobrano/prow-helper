package resolver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFindProwJobLinks(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		statusCode int
		wantLinks  []string
		wantErr    error
	}{
		{
			name: "single prow link in href",
			body: `<html><body>
				<a href="https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-job/1234567890">Job</a>
			</body></html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-job/1234567890"},
		},
		{
			name: "multiple distinct prow links",
			body: `<html><body>
				<a href="https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-a/111">Job A</a>
				<a href="https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-b/222">Job B</a>
			</body></html>`,
			statusCode: http.StatusOK,
			wantLinks: []string{
				"https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-a/111",
				"https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-b/222",
			},
		},
		{
			name: "duplicate prow links are deduplicated",
			body: `<html><body>
				<a href="https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-a/111">Link 1</a>
				<a href="https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-a/111">Link 2</a>
			</body></html>`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job-a/111"},
		},
		{
			name:       "no prow links on page",
			body:       `<html><body><p>No prow links here.</p></body></html>`,
			statusCode: http.StatusOK,
			wantErr:    ErrNoProwLinks,
		},
		{
			name:       "non-200 HTTP status",
			body:       "",
			statusCode: http.StatusNotFound,
			wantErr:    ErrFetchFailed,
		},
		{
			name: "prow link embedded in plain text",
			body: `Status: see https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/pull-ci-job/99999 for details`,
			statusCode: http.StatusOK,
			wantLinks:  []string{"https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/pull-ci-job/99999"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.body != "" {
					w.Write([]byte(tt.body)) //nolint:errcheck
				}
			}))
			defer server.Close()

			links, err := FindProwJobLinks(server.URL)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("FindProwJobLinks() expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("FindProwJobLinks() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("FindProwJobLinks() unexpected error: %v", err)
				return
			}

			if len(links) != len(tt.wantLinks) {
				t.Errorf("FindProwJobLinks() returned %d links, want %d: %v", len(links), len(tt.wantLinks), links)
				return
			}

			for i, link := range links {
				if link != tt.wantLinks[i] {
					t.Errorf("FindProwJobLinks()[%d] = %q, want %q", i, link, tt.wantLinks[i])
				}
			}
		})
	}
}
