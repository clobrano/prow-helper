package analyzer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mattn/go-shellwords"
)

// ExitError represents an error with an exit code.
type ExitError struct {
	ExitCode int
	Message  string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("analysis failed with exit code %d: %s", e.ExitCode, e.Message)
}

// ParseAnalyzeCommand parses a shell-style command string into command name and arguments.
// Handles quotes and escapes properly.
func ParseAnalyzeCommand(cmd string) (string, []string, error) {
	if strings.TrimSpace(cmd) == "" {
		return "", nil, nil
	}

	parser := shellwords.NewParser()
	parser.ParseEnv = true
	parser.ParseBacktick = false

	args, err := parser.Parse(cmd)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse command: %w", err)
	}

	if len(args) == 0 {
		return "", nil, nil
	}

	return args[0], args[1:], nil
}

// RunAnalysis executes the analysis command with the artifacts path as an argument.
// The artifacts path is appended to the command arguments.
func RunAnalysis(cmdStr, artifactsPath string) error {
	if strings.TrimSpace(cmdStr) == "" {
		// No analysis command configured, skip silently
		return nil
	}

	name, args, err := ParseAnalyzeCommand(cmdStr)
	if err != nil {
		return err
	}

	if name == "" {
		return nil
	}

	// Append artifacts path as the last argument
	args = append(args, artifactsPath)

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExitError{
				ExitCode: exitErr.ExitCode(),
				Message:  err.Error(),
			}
		}
		return fmt.Errorf("failed to run analysis command: %w", err)
	}

	return nil
}

// RunAnalysisWithIO executes the analysis command with custom IO streams.
// Useful for testing and background execution.
func RunAnalysisWithIO(cmdStr, artifactsPath string, stdout, stderr *os.File) error {
	if strings.TrimSpace(cmdStr) == "" {
		return nil
	}

	name, args, err := ParseAnalyzeCommand(cmdStr)
	if err != nil {
		return err
	}

	if name == "" {
		return nil
	}

	args = append(args, artifactsPath)

	cmd := exec.Command(name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ExitError{
				ExitCode: exitErr.ExitCode(),
				Message:  err.Error(),
			}
		}
		return fmt.Errorf("failed to run analysis command: %w", err)
	}

	return nil
}
