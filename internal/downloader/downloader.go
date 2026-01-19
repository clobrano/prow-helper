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
// Returns the user's choice.
func PromptConflictResolution(path string, stdin io.Reader, stdout io.Writer) (ConflictResolution, error) {
	fmt.Fprintf(stdout, "Folder exists: %s\n", path)
	fmt.Fprint(stdout, "[O]verwrite, [S]kip download, [N]ew timestamped folder? ")

	reader := bufio.NewReader(stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return Overwrite, err
	}

	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "o", "overwrite":
		return Overwrite, nil
	case "s", "skip":
		return Skip, nil
	case "n", "new":
		return NewTimestamped, nil
	default:
		// Default to overwrite
		return Overwrite, nil
	}
}

// ResolveDestination handles the full destination resolution including conflict handling.
func ResolveDestination(baseDest string, metadata *parser.ProwMetadata, stdin io.Reader, stdout io.Writer) (string, bool, error) {
	destPath := BuildDestinationPath(baseDest, metadata)

	exists, err := CheckDestinationConflict(destPath)
	if err != nil {
		return "", false, err
	}

	if !exists {
		return destPath, false, nil
	}

	resolution, err := PromptConflictResolution(destPath, stdin, stdout)
	if err != nil {
		return "", false, err
	}

	switch resolution {
	case Skip:
		return destPath, true, nil
	case NewTimestamped:
		return CreateTimestampedPath(destPath), false, nil
	default: // Overwrite
		// Remove existing directory
		if err := os.RemoveAll(destPath); err != nil {
			return "", false, fmt.Errorf("failed to remove existing directory: %w", err)
		}
		return destPath, false, nil
	}
}
