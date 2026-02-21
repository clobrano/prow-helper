package analyzer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

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

// execSyscall is the low-level exec function used to replace the current process.
// It is a variable so tests can override it without actually replacing the test process.
var execSyscall = syscall.Exec

// osChdir is a variable so tests can override it without mutating the test process's cwd.
var osChdir = os.Chdir

// RunAnalysis replaces the current process with the analysis command by using
// the exec syscall. The artifacts path is appended as the last argument and the
// working directory is changed to artifactsPath before exec, so any files the
// analysis command writes land in the same folder as the downloaded data.
// Because exec replaces the process in-place (same PID, terminal, and process
// group), the session runs directly in the current shell — plain terminal or
// tmux pane — with no intermediate child process.
//
// RunAnalysis only returns when the exec itself fails (e.g. command not found).
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

	// Resolve the full executable path
	execPath, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("command not found %q: %w", name, err)
	}

	// Change into the artifacts directory so any files the analysis command
	// writes (using relative paths) land alongside the downloaded data.
	if err := osChdir(artifactsPath); err != nil {
		return fmt.Errorf("failed to chdir to artifacts path %q: %w", artifactsPath, err)
	}

	// Replace the current process with the analysis command.
	// argv[0] is conventionally the program name, followed by the arguments.
	return execSyscall(execPath, append([]string{name}, args...), os.Environ())
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
	cmd.Dir = artifactsPath
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
