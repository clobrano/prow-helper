package resolver

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

var (
	ErrFetchFailed   = errors.New("failed to fetch URL")
	ErrNoProwLinks   = errors.New("no prow job links found on page")

	// prowLinkPattern matches prow.ci.openshift.org /view/gs/ URLs embedded in HTML
	prowLinkPattern = regexp.MustCompile(`https://prow\.ci\.openshift\.org/view/gs/[^\s"'<>]+`)
)

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
