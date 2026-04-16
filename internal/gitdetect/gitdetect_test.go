package gitdetect

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestParseGitHubRemote(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		wantOrg   string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "SSH format",
			remoteURL: "git@github.com:openshift/cno.git",
			wantOrg:   "openshift",
			wantRepo:  "cno",
		},
		{
			name:      "SSH format without .git",
			remoteURL: "git@github.com:openshift/cno",
			wantOrg:   "openshift",
			wantRepo:  "cno",
		},
		{
			name:      "HTTPS format",
			remoteURL: "https://github.com/openshift/cno.git",
			wantOrg:   "openshift",
			wantRepo:  "cno",
		},
		{
			name:      "HTTPS format without .git",
			remoteURL: "https://github.com/openshift/cno",
			wantOrg:   "openshift",
			wantRepo:  "cno",
		},
		{
			name:      "HTTPS format with trailing slash",
			remoteURL: "https://github.com/openshift/cno/",
			wantOrg:   "openshift",
			wantRepo:  "cno",
		},
		{
			name:      "HTTP format",
			remoteURL: "http://github.com/openshift/release.git",
			wantOrg:   "openshift",
			wantRepo:  "release",
		},
		{
			name:      "SSH URL format",
			remoteURL: "ssh://git@github.com/openshift/cno.git",
			wantOrg:   "openshift",
			wantRepo:  "cno",
		},
		{
			name:      "SSH URL format without .git",
			remoteURL: "ssh://git@github.com/openshift/cno",
			wantOrg:   "openshift",
			wantRepo:  "cno",
		},
		{
			name:      "non-GitHub remote",
			remoteURL: "git@gitlab.com:openshift/cno.git",
			wantErr:   true,
		},
		{
			name:      "non-GitHub HTTPS remote",
			remoteURL: "https://gitlab.com/openshift/cno.git",
			wantErr:   true,
		},
		{
			name:      "empty string",
			remoteURL: "",
			wantErr:   true,
		},
		{
			name:      "bare path",
			remoteURL: "/home/user/repo.git",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			org, repo, err := ParseGitHubRemote(tt.remoteURL)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseGitHubRemote(%q) expected error, got org=%q repo=%q", tt.remoteURL, org, repo)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseGitHubRemote(%q) unexpected error: %v", tt.remoteURL, err)
			}
			if org != tt.wantOrg {
				t.Errorf("ParseGitHubRemote(%q) org = %q, want %q", tt.remoteURL, org, tt.wantOrg)
			}
			if repo != tt.wantRepo {
				t.Errorf("ParseGitHubRemote(%q) repo = %q, want %q", tt.remoteURL, repo, tt.wantRepo)
			}
		})
	}
}

// fakeExecCommand creates a mock exec.Command that delegates to TestHelperProcess.
func fakeExecCommand(responses map[string]struct {
	stdout   string
	exitCode int
}) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		key := name + " " + strings.Join(args, " ")
		resp, ok := responses[key]
		if !ok {
			// Return a command that will fail
			cs := []string{"-test.run=TestHelperProcess", "--", "FAIL"}
			cmd := exec.Command(os.Args[0], cs...)
			cmd.Env = append(os.Environ(), "GO_TEST_HELPER_PROCESS=1", "GO_TEST_EXIT_CODE=1", "GO_TEST_STDOUT=")
			return cmd
		}
		cs := []string{"-test.run=TestHelperProcess", "--"}
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(),
			"GO_TEST_HELPER_PROCESS=1",
			fmt.Sprintf("GO_TEST_EXIT_CODE=%d", resp.exitCode),
			"GO_TEST_STDOUT="+resp.stdout,
		)
		return cmd
	}
}

// TestHelperProcess is used by fakeExecCommand to simulate command output.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_PROCESS") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, os.Getenv("GO_TEST_STDOUT"))
	exitCode := os.Getenv("GO_TEST_EXIT_CODE")
	if exitCode == "1" {
		os.Exit(1)
	}
	os.Exit(0)
}

func TestDetectPRURL_ViaGH(t *testing.T) {
	original := ExecCommand
	defer func() { ExecCommand = original }()

	ExecCommand = fakeExecCommand(map[string]struct {
		stdout   string
		exitCode int
	}{
		"git rev-parse --is-inside-work-tree": {stdout: "true\n", exitCode: 0},
		"gh pr view --json url -q .url":       {stdout: "https://github.com/openshift/cno/pull/42\n", exitCode: 0},
	})

	url, err := DetectPRURL()
	if err != nil {
		t.Fatalf("DetectPRURL() unexpected error: %v", err)
	}
	if url != "https://github.com/openshift/cno/pull/42" {
		t.Errorf("DetectPRURL() = %q, want %q", url, "https://github.com/openshift/cno/pull/42")
	}
}

func TestDetectPRURL_NotGitRepo(t *testing.T) {
	original := ExecCommand
	defer func() { ExecCommand = original }()

	ExecCommand = fakeExecCommand(map[string]struct {
		stdout   string
		exitCode int
	}{
		"git rev-parse --is-inside-work-tree": {stdout: "", exitCode: 1},
	})

	_, err := DetectPRURL()
	if err == nil {
		t.Error("DetectPRURL() expected error for non-git directory, got nil")
	}
}

func TestDetectPRURL_FallbackToGitHubAPI(t *testing.T) {
	// Set up a test HTTP server that returns a PR
	prs := []githubPR{{HTMLURL: "https://github.com/openshift/cno/pull/99", Number: 99}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(prs) //nolint:errcheck
	}))
	defer server.Close()

	originalExec := ExecCommand
	originalClient := HTTPClient
	defer func() {
		ExecCommand = originalExec
		HTTPClient = originalClient
	}()

	ExecCommand = fakeExecCommand(map[string]struct {
		stdout   string
		exitCode int
	}{
		"git rev-parse --is-inside-work-tree": {stdout: "true\n", exitCode: 0},
		// gh fails -> triggers fallback
		"gh pr view --json url -q .url":  {stdout: "", exitCode: 1},
		"git remote get-url origin":      {stdout: "https://github.com/openshift/cno.git\n", exitCode: 0},
		"git branch --show-current":      {stdout: "my-feature-branch\n", exitCode: 0},
	})

	// We need to intercept the GitHub API call. Replace queryGitHubPR behavior
	// by overriding the HTTP client to redirect to our test server.
	HTTPClient = server.Client()
	// The test server URL is different from the GitHub API URL, so we use a
	// custom transport to redirect requests.
	HTTPClient.Transport = &rewriteTransport{
		base:    http.DefaultTransport,
		testURL: server.URL,
	}

	url, err := DetectPRURL()
	if err != nil {
		t.Fatalf("DetectPRURL() unexpected error: %v", err)
	}
	if url != "https://github.com/openshift/cno/pull/99" {
		t.Errorf("DetectPRURL() = %q, want %q", url, "https://github.com/openshift/cno/pull/99")
	}
}

func TestDetectPRURL_NoPRFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]githubPR{}) //nolint:errcheck
	}))
	defer server.Close()

	originalExec := ExecCommand
	originalClient := HTTPClient
	defer func() {
		ExecCommand = originalExec
		HTTPClient = originalClient
	}()

	ExecCommand = fakeExecCommand(map[string]struct {
		stdout   string
		exitCode int
	}{
		"git rev-parse --is-inside-work-tree": {stdout: "true\n", exitCode: 0},
		"gh pr view --json url -q .url":       {stdout: "", exitCode: 1},
		"git remote get-url origin":           {stdout: "git@github.com:openshift/cno.git\n", exitCode: 0},
		"git branch --show-current":           {stdout: "no-pr-branch\n", exitCode: 0},
	})

	HTTPClient = server.Client()
	HTTPClient.Transport = &rewriteTransport{
		base:    http.DefaultTransport,
		testURL: server.URL,
	}

	_, err := DetectPRURL()
	if err == nil {
		t.Error("DetectPRURL() expected error when no PR exists, got nil")
	}
}

func TestDetectPRURL_DetachedHEAD(t *testing.T) {
	originalExec := ExecCommand
	defer func() { ExecCommand = originalExec }()

	ExecCommand = fakeExecCommand(map[string]struct {
		stdout   string
		exitCode int
	}{
		"git rev-parse --is-inside-work-tree": {stdout: "true\n", exitCode: 0},
		"gh pr view --json url -q .url":       {stdout: "", exitCode: 1},
		"git remote get-url origin":           {stdout: "https://github.com/openshift/cno.git\n", exitCode: 0},
		"git branch --show-current":           {stdout: "\n", exitCode: 0},
	})

	_, err := DetectPRURL()
	if err == nil {
		t.Error("DetectPRURL() expected error for detached HEAD, got nil")
	}
}

func TestDetectPRURL_NonGitHubRemote(t *testing.T) {
	originalExec := ExecCommand
	defer func() { ExecCommand = originalExec }()

	ExecCommand = fakeExecCommand(map[string]struct {
		stdout   string
		exitCode int
	}{
		"git rev-parse --is-inside-work-tree": {stdout: "true\n", exitCode: 0},
		"gh pr view --json url -q .url":       {stdout: "", exitCode: 1},
		"git remote get-url origin":           {stdout: "git@gitlab.com:openshift/cno.git\n", exitCode: 0},
	})

	_, err := DetectPRURL()
	if err == nil {
		t.Error("DetectPRURL() expected error for non-GitHub remote, got nil")
	}
}

// rewriteTransport redirects all requests to a test server URL.
type rewriteTransport struct {
	base    http.RoundTripper
	testURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the request URL to point to the test server
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.testURL, "http://")
	return t.base.RoundTrip(req)
}
