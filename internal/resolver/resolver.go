package resolver

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
)

var (
	ErrFetchFailed   = errors.New("failed to fetch URL")
	ErrNoProwLinks   = errors.New("no prow job links found on page")
	ErrInvalidPRURL  = errors.New("invalid GitHub PR URL")

	// prowLinkPattern matches prow.ci.openshift.org /view/gs/ URLs embedded in HTML
	prowLinkPattern = regexp.MustCompile(`https://prow\.ci\.openshift\.org/view/gs/[^\s"'<>]+`)

	// githubPRPattern matches GitHub pull request URLs: github.com/<org>/<repo>/pull/<number>
	githubPRPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+)/pull/(\d+)(?:[/?#].*)?$`)
)

// GitHubPR holds the components of a GitHub pull request URL.
type GitHubPR struct {
	Org    string
	Repo   string
	Number int
}

// ParseGitHubPRURL parses a GitHub PR URL and extracts org, repo, and PR number.
// Returns nil if the URL is not a GitHub PR URL.
func ParseGitHubPRURL(rawURL string) *GitHubPR {
	matches := githubPRPattern.FindStringSubmatch(rawURL)
	if matches == nil {
		return nil
	}
	num, _ := strconv.Atoi(matches[3]) // regex guarantees digits
	return &GitHubPR{
		Org:    matches[1],
		Repo:   matches[2],
		Number: num,
	}
}

// FindProwJobLinks fetches the given URL and returns all prow job links found on the page.
// Returns ErrNoProwLinks if the page contains no recognizable prow job URLs.
func FindProwJobLinks(url string) ([]string, error) {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrFetchFailed, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: reading body: %v", ErrFetchFailed, err)
	}

	matches := prowLinkPattern.FindAllString(string(body), -1)
	if len(matches) == 0 {
		return nil, ErrNoProwLinks
	}

	return deduplicate(matches), nil
}

// deduplicate returns a slice with duplicate strings removed, preserving order.
func deduplicate(links []string) []string {
	seen := make(map[string]bool, len(links))
	result := make([]string, 0, len(links))
	for _, link := range links {
		if !seen[link] {
			seen[link] = true
			result = append(result, link)
		}
	}
	return result
}
