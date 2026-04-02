package resolver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseGitHubPRURL(t *testing.T) {
	tests := []struct {
		name   string
		url    string
		want   *GitHubPR
	}{
		{
			name: "standard PR URL",
			url:  "https://github.com/openshift/cno/pull/42",
			want: &GitHubPR{Org: "openshift", Repo: "cno", Number: 42},
		},
		{
			name: "PR URL with trailing slash",
			url:  "https://github.com/openshift/release/pull/123/",
			want: &GitHubPR{Org: "openshift", Repo: "release", Number: 123},
		},
		{
			name: "PR URL with files tab",
			url:  "https://github.com/openshift/cno/pull/42/files",
			want: &GitHubPR{Org: "openshift", Repo: "cno", Number: 42},
		},
		{
			name: "PR URL with checks tab",
			url:  "https://github.com/openshift/cno/pull/42/checks",
			want: &GitHubPR{Org: "openshift", Repo: "cno", Number: 42},
		},
		{
			name: "PR URL with query params",
			url:  "https://github.com/openshift/cno/pull/42?diff=unified",
			want: &GitHubPR{Org: "openshift", Repo: "cno", Number: 42},
		},
		{
			name: "PR URL with fragment",
			url:  "https://github.com/openshift/cno/pull/42#discussion_r123",
			want: &GitHubPR{Org: "openshift", Repo: "cno", Number: 42},
		},
		{
			name: "HTTP (not HTTPS)",
			url:  "http://github.com/openshift/cno/pull/42",
			want: &GitHubPR{Org: "openshift", Repo: "cno", Number: 42},
		},
		{
			name: "not a PR URL - issue",
			url:  "https://github.com/openshift/cno/issues/42",
			want: nil,
		},
		{
			name: "not a PR URL - repo root",
			url:  "https://github.com/openshift/cno",
			want: nil,
		},
		{
			name: "not a PR URL - prow URL",
			url:  "https://prow.ci.openshift.org/view/gs/test-platform-results/logs/job/123",
			want: nil,
		},
		{
			name: "not a PR URL - empty",
			url:  "",
			want: nil,
		},
		{
			name: "not a PR URL - non-numeric PR number",
			url:  "https://github.com/openshift/cno/pull/abc",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseGitHubPRURL(tt.url)
			if tt.want == nil {
				if got != nil {
					t.Errorf("ParseGitHubPRURL(%q) = %+v, want nil", tt.url, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseGitHubPRURL(%q) = nil, want %+v", tt.url, tt.want)
			}
			if got.Org != tt.want.Org || got.Repo != tt.want.Repo || got.Number != tt.want.Number {
				t.Errorf("ParseGitHubPRURL(%q) = %+v, want %+v", tt.url, got, tt.want)
			}
		})
	}
}

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
