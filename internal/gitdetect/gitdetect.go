package gitdetect

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
)

// ExecCommand is the function used to create exec.Cmd objects.
// Override in tests to mock command execution.
var ExecCommand = exec.Command

// HTTPClient is the HTTP client used for GitHub API requests.
// Override in tests to use a custom transport.
var HTTPClient = &http.Client{}

var (
	// sshRemotePattern matches git@github.com:org/repo.git
	sshRemotePattern = regexp.MustCompile(`^git@github\.com:([^/]+)/([^/.]+?)(?:\.git)?$`)
	// httpsRemotePattern matches https://github.com/org/repo.git or https://github.com/org/repo
	httpsRemotePattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/.]+?)(?:\.git)?/?$`)
	// sshURLRemotePattern matches ssh://git@github.com/org/repo.git
	sshURLRemotePattern = regexp.MustCompile(`^ssh://git@github\.com/([^/]+)/([^/.]+?)(?:\.git)?/?$`)
)

// DetectPRURL attempts to find a GitHub PR URL associated with the current
// git directory. It tries using the gh CLI first, then falls back to parsing
// the git remote and querying the GitHub API.
func DetectPRURL() (string, error) {
	if err := checkGitRepo(); err != nil {
		return "", err
	}

	// Try gh CLI first (handles auth, private repos, etc.)
	if url, err := detectViaGH(); err == nil && url != "" {
		return url, nil
	}

	// Fall back to git remote + GitHub API
	return detectViaGitRemote()
}

// checkGitRepo verifies we are inside a git working tree.
func checkGitRepo() error {
	cmd := ExecCommand("git", "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("not inside a git repository")
	}
	if strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("not inside a git repository")
	}
	return nil
}

// detectViaGH uses the gh CLI to find the PR for the current branch.
func detectViaGH() (string, error) {
	cmd := ExecCommand("gh", "pr", "view", "--json", "url", "-q", ".url")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh pr view failed: %w", err)
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", fmt.Errorf("no PR found via gh")
	}
	return url, nil
}

// detectViaGitRemote parses the git remote URL to extract org/repo, gets the
// current branch name, and queries the GitHub API to find an open PR.
func detectViaGitRemote() (string, error) {
	cmd := ExecCommand("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git remote URL: %w", err)
	}
	remoteURL := strings.TrimSpace(string(out))

	org, repo, err := ParseGitHubRemote(remoteURL)
	if err != nil {
		return "", err
	}

	cmd = ExecCommand("git", "branch", "--show-current")
	out, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", fmt.Errorf("detached HEAD state, cannot detect PR")
	}

	return queryGitHubPR(org, repo, branch)
}

// ParseGitHubRemote extracts the org and repo from a GitHub remote URL.
// Supports SSH (git@github.com:org/repo.git), HTTPS, and ssh:// URL formats.
func ParseGitHubRemote(remoteURL string) (org, repo string, err error) {
	patterns := []*regexp.Regexp{sshRemotePattern, httpsRemotePattern, sshURLRemotePattern}
	for _, p := range patterns {
		matches := p.FindStringSubmatch(remoteURL)
		if matches != nil {
			return matches[1], matches[2], nil
		}
	}
	return "", "", fmt.Errorf("could not parse GitHub org/repo from remote URL: %s", remoteURL)
}

// githubPR represents a PR returned by the GitHub API.
type githubPR struct {
	HTMLURL string `json:"html_url"`
	Number  int    `json:"number"`
}

// queryGitHubPR queries the GitHub API for an open PR matching the given branch.
func queryGitHubPR(org, repo, branch string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?head=%s:%s&state=open",
		org, repo, org, branch)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var prs []githubPR
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return "", fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	if len(prs) == 0 {
		return "", fmt.Errorf("no open PR found for branch %q", branch)
	}

	return prs[0].HTMLURL, nil
}
