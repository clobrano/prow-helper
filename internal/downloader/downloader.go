package downloader

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/clobrano/prow-helper/internal/parser"
)

var (
	ErrGsutilNotFound    = errors.New("gsutil command not found. Please install Google Cloud SDK")
	ErrDownloadFailed    = errors.New("failed to download artifacts")
	ErrDestinationExists = errors.New("destination folder already exists")
)

// ConflictResolution represents the user's choice when destination exists.
type ConflictResolution int

const (
	Overwrite ConflictResolution = iota
	Skip
	NewTimestamped
)

// ConflictPolicy selects how an existing destination folder is handled.
// PolicyPrompt asks the user interactively; the other policies resolve the
// conflict without asking.
type ConflictPolicy string

const (
	PolicyPrompt    ConflictPolicy = "prompt"
	PolicyOverwrite ConflictPolicy = "overwrite"
	PolicySkip      ConflictPolicy = "skip"
	PolicyNew       ConflictPolicy = "new"
)

// ParseConflictPolicy validates a policy string (e.g. from a CLI flag or
// config file) and returns the corresponding ConflictPolicy.
func ParseConflictPolicy(s string) (ConflictPolicy, error) {
	switch ConflictPolicy(s) {
	case PolicyPrompt, PolicyOverwrite, PolicySkip, PolicyNew:
		return ConflictPolicy(s), nil
	default:
		return "", fmt.Errorf("invalid conflict policy %q (valid: prompt, overwrite, skip, new)", s)
	}
}

// BuildDestinationPath constructs the full destination path for artifacts.
// Format: <baseDest>/<job-name>/<build-id>/
func BuildDestinationPath(baseDest string, metadata *parser.ProwMetadata) string {
	// Expand ~ to home directory if present
	if strings.HasPrefix(baseDest, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			baseDest = filepath.Join(home, baseDest[2:])
		}
	}
	return filepath.Join(baseDest, metadata.JobName, metadata.BuildID)
}

// CheckDestinationConflict checks if the destination folder already exists.
func CheckDestinationConflict(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

// CreateTimestampedPath creates a new path with a timestamp suffix.
func CreateTimestampedPath(basePath string) string {
	timestamp := time.Now().Format("20060102-150405")
	return basePath + "-" + timestamp
}

// CheckGsutilAvailable verifies that gsutil is installed and accessible.
func CheckGsutilAvailable() error {
	_, err := exec.LookPath("gsutil")
	if err != nil {
		return ErrGsutilNotFound
	}
	return nil
}

// Download executes the gsutil command to download artifacts.
// It streams output to the provided writers for progress indication.
func Download(gcsPath, destPath string, stdout, stderr io.Writer) error {
	if err := CheckGsutilAvailable(); err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Build gsutil command
	// gsutil -m cp -r gs://<bucket>/<path>/* <dest>
	cmd := exec.Command("gsutil", "-m", "cp", "-r", gcsPath+"/*", destPath)

	// Set up pipes for output
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start gsutil: %w", err)
	}

	// Stream output
	go streamOutput(stdoutPipe, stdout)
	go streamOutput(stderrPipe, stderr)

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%w: %v", ErrDownloadFailed, err)
	}

	return nil
}

// streamOutput reads from reader and writes to writer line by line.
func streamOutput(reader io.Reader, writer io.Writer) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fmt.Fprintln(writer, scanner.Text())
	}
}

// PromptConflictResolution prompts the user to choose how to handle an existing folder.
// An empty answer defaults to Skip (the safe choice); any other unrecognized
// input re-prompts, so that a typo can never trigger a destructive overwrite.
func PromptConflictResolution(path string, stdin io.Reader, stdout io.Writer) (ConflictResolution, error) {
	fmt.Fprintf(stdout, "Folder exists: %s\n", path)

	reader := bufio.NewReader(stdin)
	for {
		fmt.Fprint(stdout, "[O]verwrite, [S]kip download, [N]ew timestamped folder? [S]: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return Skip, err
		}

		switch strings.TrimSpace(strings.ToLower(input)) {
		case "o", "overwrite":
			return Overwrite, nil
		case "s", "skip", "":
			return Skip, nil
		case "n", "new":
			return NewTimestamped, nil
		default:
			fmt.Fprintln(stdout, "Please answer o, s, or n.")
		}
	}
}

// ResolveDestination handles the full destination resolution, prompting the
// user interactively when the destination already exists.
func ResolveDestination(baseDest string, metadata *parser.ProwMetadata, stdin io.Reader, stdout io.Writer) (string, bool, error) {
	return ResolveDestinationWithPolicy(baseDest, metadata, PolicyPrompt, stdin, stdout)
}

// ResolveDestinationWithPolicy resolves the destination path, handling an
// existing folder according to policy. PolicyPrompt asks the user; the other
// policies resolve the conflict without any interaction, which makes them
// safe for unattended runs (background mode, long watch sessions).
// The returned bool is true when the download should be skipped because the
// existing artifacts are reused.
func ResolveDestinationWithPolicy(baseDest string, metadata *parser.ProwMetadata, policy ConflictPolicy, stdin io.Reader, stdout io.Writer) (string, bool, error) {
	destPath := BuildDestinationPath(baseDest, metadata)

	exists, err := CheckDestinationConflict(destPath)
	if err != nil {
		return "", false, err
	}

	if !exists {
		return destPath, false, nil
	}

	resolution, err := ResolveConflictAction(destPath, policy, stdin, stdout)
	if err != nil {
		return "", false, err
	}
	return ApplyResolution(destPath, resolution)
}

// ResolveConflictAction decides how an existing destination should be handled
// under the given policy, prompting the user only for PolicyPrompt. It does
// not touch the filesystem; pass the result to ApplyResolution when the
// download is about to start. This split lets callers ask the question early
// (e.g. before a long watch) while deferring any deletion until it is needed.
func ResolveConflictAction(destPath string, policy ConflictPolicy, stdin io.Reader, stdout io.Writer) (ConflictResolution, error) {
	switch policy {
	case PolicyOverwrite:
		return Overwrite, nil
	case PolicySkip:
		return Skip, nil
	case PolicyNew:
		return NewTimestamped, nil
	default: // PolicyPrompt
		return PromptConflictResolution(destPath, stdin, stdout)
	}
}

// ApplyResolution applies a previously chosen ConflictResolution to destPath,
// returning the final destination and whether the download should be skipped.
// For Overwrite the existing folder is removed here.
func ApplyResolution(destPath string, resolution ConflictResolution) (string, bool, error) {
	switch resolution {
	case Skip:
		return destPath, true, nil
	case NewTimestamped:
		return CreateTimestampedPath(destPath), false, nil
	default: // Overwrite
		if err := os.RemoveAll(destPath); err != nil {
			return "", false, fmt.Errorf("failed to remove existing directory: %w", err)
		}
		return destPath, false, nil
	}
}
