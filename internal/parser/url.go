package parser

import (
	"errors"
	"net/url"
	"strings"
)

const (
	prowHost   = "prow.ci.openshift.org"
	pathPrefix = "/view/gs/"
)

var (
	ErrEmptyURL        = errors.New("URL cannot be empty")
	ErrInvalidURL      = errors.New("invalid URL format")
	ErrInvalidHost     = errors.New("invalid host: expected prow.ci.openshift.org")
	ErrInvalidScheme   = errors.New("invalid scheme: expected https")
	ErrInvalidPath     = errors.New("invalid path: expected /view/gs/<bucket>/<path>")
	ErrMissingPath     = errors.New("missing required path components")
)

// ProwMetadata contains the extracted information from a PROW URL.
type ProwMetadata struct {
	Bucket   string // GCS bucket name (e.g., "test-platform-results")
	Path     string // Full GCS path after bucket (e.g., "logs/job-name/build-id")
	JobName  string // Job name extracted from path
	BuildID  string // Build ID (last component of path)
	PRRef    string // "[org/repo PR<num>]" for PR jobs, empty for others
	RawURL   string // Original URL
}

// ValidateURL validates that the given URL is a valid PROW URL.
// Expected format: https://prow.ci.openshift.org/view/gs/<bucket>/<path>/<build-id>
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return ErrEmptyURL
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ErrInvalidURL
	}

	if parsed.Scheme != "https" {
		return ErrInvalidScheme
	}

	if parsed.Host != prowHost {
		return ErrInvalidHost
	}

	if !strings.HasPrefix(parsed.Path, pathPrefix) {
		return ErrInvalidPath
	}

	// Extract the path after /view/gs/
	gcsPath := strings.TrimPrefix(parsed.Path, pathPrefix)
	gcsPath = strings.TrimSuffix(gcsPath, "/")

	// Need at least bucket/path/build-id (3 components minimum)
	parts := strings.Split(gcsPath, "/")
	if len(parts) < 3 {
		return ErrMissingPath
	}

	// Check that bucket is not empty
	if parts[0] == "" {
		return ErrMissingPath
	}

	return nil
}

// ParseURL parses a PROW URL and extracts metadata.
// Returns a ProwMetadata struct with bucket, path, job name, and build ID.
func ParseURL(rawURL string) (*ProwMetadata, error) {
	if err := ValidateURL(rawURL); err != nil {
		return nil, err
	}

	parsed, _ := url.Parse(rawURL) // Already validated, ignore error

	// Extract the path after /view/gs/
	gcsPath := strings.TrimPrefix(parsed.Path, pathPrefix)
	gcsPath = strings.TrimSuffix(gcsPath, "/")

	parts := strings.Split(gcsPath, "/")

	// First part is the bucket
	bucket := parts[0]

	// Rest is the path
	path := strings.Join(parts[1:], "/")

	// Build ID is the last component
	buildID := parts[len(parts)-1]

	// Job name is the second-to-last component
	jobName := parts[len(parts)-2]

	// Extract PR reference for pr-logs paths:
	// pr-logs/pull/<org_repo>/<pr_num>/<job_name>/<build_id>
	var prRef string
	if len(parts) >= 6 && parts[1] == "pr-logs" && parts[2] == "pull" {
		orgRepo := parts[3]
		prNum := parts[4]
		orgRepoParts := strings.SplitN(orgRepo, "_", 2)
		if len(orgRepoParts) == 2 && prNum != "" {
			prRef = "[" + orgRepoParts[0] + "/" + orgRepoParts[1] + " PR" + prNum + "]"
		}
	}

	return &ProwMetadata{
		Bucket:  bucket,
		Path:    path,
		JobName: jobName,
		BuildID: buildID,
		PRRef:   prRef,
		RawURL:  rawURL,
	}, nil
}

// BuildGsutilCommand constructs the gsutil command to download artifacts.
// Returns the full command string: gsutil -m cp -r gs://<bucket>/<path>/ <dest>
func BuildGsutilCommand(metadata *ProwMetadata, dest string) string {
	return "gsutil -m cp -r gs://" + metadata.Bucket + "/" + metadata.Path + "/ " + dest
}
