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

// RunAnalysis executes the analysis command with the artifacts path appended as
// the last argument.
//
// When interactive is true the current process is replaced by the analysis
// command via the exec syscall (same PID, terminal, and process group), so the
// session runs directly in the current shell with no intermediate child process.
// RunAnalysis only returns in this mode when the exec itself fails.
//
// When interactive is false the command is run as a normal child process with
// stdin/stdout/stderr connected to the current terminal.
func RunAnalysis(cmdStr, artifactsPath string, interactive bool) error {
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

	if interactive {
		// Resolve the full executable path
		execPath, err := exec.LookPath(name)
		if err != nil {
			return fmt.Errorf("command not found %q: %w", name, err)
		}

		// Replace the current process with the analysis command.
		// argv[0] is conventionally the program name, followed by the arguments.
		return execSyscall(execPath, append([]string{name}, args...), os.Environ())
	}

	// Non-interactive: run as a child process with I/O connected to the terminal.
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
