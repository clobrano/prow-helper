package analyzer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAnalyzeCommand(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string
		wantName string
		wantArgs []string
		wantErr  bool
	}{
		{
			name:     "simple command",
			cmd:      "echo",
			wantName: "echo",
			wantArgs: []string{},
			wantErr:  false,
		},
		{
			name:     "command with arguments",
			cmd:      "echo hello world",
			wantName: "echo",
			wantArgs: []string{"hello", "world"},
			wantErr:  false,
		},
		{
			name:     "command with quoted arguments",
			cmd:      `echo "hello world"`,
			wantName: "echo",
			wantArgs: []string{"hello world"},
			wantErr:  false,
		},
		{
			name:     "command with single quotes",
			cmd:      `echo 'hello world'`,
			wantName: "echo",
			wantArgs: []string{"hello world"},
			wantErr:  false,
		},
		{
			name:     "command with mixed quotes",
			cmd:      `mycommand --flag "value with spaces" --other 'single quoted'`,
			wantName: "mycommand",
			wantArgs: []string{"--flag", "value with spaces", "--other", "single quoted"},
			wantErr:  false,
		},
		{
			name:     "empty command",
			cmd:      "",
			wantName: "",
			wantArgs: nil,
			wantErr:  false,
		},
		{
			name:     "whitespace only",
			cmd:      "   ",
			wantName: "",
			wantArgs: nil,
			wantErr:  false,
		},
		{
			name:     "path with spaces",
			cmd:      `"/path/to/my command" arg1`,
			wantName: "/path/to/my command",
			wantArgs: []string{"arg1"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, args, err := ParseAnalyzeCommand(tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAnalyzeCommand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if name != tt.wantName {
				t.Errorf("ParseAnalyzeCommand() name = %v, want %v", name, tt.wantName)
			}
			if len(args) != len(tt.wantArgs) {
				t.Errorf("ParseAnalyzeCommand() args = %v, want %v", args, tt.wantArgs)
				return
			}
			for i, arg := range args {
				if arg != tt.wantArgs[i] {
					t.Errorf("ParseAnalyzeCommand() args[%d] = %v, want %v", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}

func TestRunAnalysis_EmptyCommand(t *testing.T) {
	if err := RunAnalysis("", "/some/path"); err != nil {
		t.Errorf("RunAnalysis() with empty command should not error, got %v", err)
	}
}

func TestRunAnalysis_WhitespaceCommand(t *testing.T) {
	if err := RunAnalysis("   ", "/some/path"); err != nil {
		t.Errorf("RunAnalysis() with whitespace command should not error, got %v", err)
	}
}

// mockExecSyscall replaces execSyscall for the duration of a test and restores it afterward.
// It captures the arguments that would have been passed to exec and returns the provided error.
func mockExecSyscall(t *testing.T, returnErr error) (path *string, argv *[]string) {
	t.Helper()
	var capturedPath string
	var capturedArgv []string

	orig := execSyscall
	t.Cleanup(func() { execSyscall = orig })

	execSyscall = func(p string, av []string, _ []string) error {
		capturedPath = p
		capturedArgv = av
		return returnErr
	}

	return &capturedPath, &capturedArgv
}

func TestRunAnalysis_ExecsInCurrentShell(t *testing.T) {
	// RunAnalysis must use exec (not fork+exec) so the session runs in the
	// current shell with no intermediate child process.
	gotPath, gotArgv := mockExecSyscall(t, nil)

	tmpDir := t.TempDir()
	if err := RunAnalysis("echo", tmpDir); err != nil {
		t.Errorf("RunAnalysis() error = %v, want nil", err)
	}

	if *gotPath == "" {
		t.Fatal("execSyscall was not called")
	}

	// argv[0] should be the command name, last element should be the artifacts path
	if (*gotArgv)[0] != "echo" {
		t.Errorf("argv[0] = %q, want %q", (*gotArgv)[0], "echo")
	}
	if (*gotArgv)[len(*gotArgv)-1] != tmpDir {
		t.Errorf("last argv = %q, want artifacts path %q", (*gotArgv)[len(*gotArgv)-1], tmpDir)
	}
}

func TestRunAnalysis_PassesArtifactsPath(t *testing.T) {
	_, gotArgv := mockExecSyscall(t, nil)

	artifactsPath := "/test/artifacts/path"
	if err := RunAnalysis("echo", artifactsPath); err != nil {
		t.Fatalf("RunAnalysis() error = %v", err)
	}

	last := (*gotArgv)[len(*gotArgv)-1]
	if last != artifactsPath {
		t.Errorf("artifacts path in argv = %q, want %q", last, artifactsPath)
	}
}

func TestRunAnalysis_NonExistentCommand(t *testing.T) {
	// LookPath should fail before execSyscall is called
	err := RunAnalysis("nonexistent-command-12345", "/some/path")
	if err == nil {
		t.Error("RunAnalysis() should return error for non-existent command")
	}
}

func TestRunAnalysis_ExecError(t *testing.T) {
	// If execSyscall returns an error (e.g. permission denied), RunAnalysis propagates it.
	wantErr := errors.New("exec: permission denied")
	_, _ = mockExecSyscall(t, wantErr)

	// "echo" is a real command so LookPath succeeds; the mock then returns the error.
	err := RunAnalysis("echo", "/some/path")
	if err == nil {
		t.Fatal("RunAnalysis() should return error when execSyscall fails")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("RunAnalysis() error = %v, want %v", err, wantErr)
	}
}

func TestExitError(t *testing.T) {
	err := &ExitError{
		ExitCode: 42,
		Message:  "test error",
	}

	errStr := err.Error()
	if errStr != "analysis failed with exit code 42: test error" {
		t.Errorf("ExitError.Error() = %v, unexpected format", errStr)
	}
}

func TestRunAnalysis_CommandWithExtraArgs(t *testing.T) {
	// Verify that extra args from the command string appear before the artifacts path.
	_, gotArgv := mockExecSyscall(t, nil)

	artifactsPath := "/artifacts"
	// Use "echo" (always in PATH) with extra flags to test arg ordering.
	if err := RunAnalysis("echo --flag value", artifactsPath); err != nil {
		t.Fatalf("RunAnalysis() error = %v", err)
	}

	// Expected argv: ["echo", "--flag", "value", "/artifacts"]
	av := *gotArgv
	if av[0] != "echo" {
		t.Errorf("argv[0] = %q, want %q", av[0], "echo")
	}
	if av[len(av)-1] != artifactsPath {
		t.Errorf("last argv = %q, want %q", av[len(av)-1], artifactsPath)
	}
}

// TestRunAnalysisWithIO_PassesArtifactsPath keeps the RunAnalysisWithIO behaviour tested
// via an actual subprocess (exec.Command), which is used for background/testing scenarios.
func TestRunAnalysisWithIO_PassesArtifactsPath(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	scriptContent := `#!/bin/bash
echo "$1" > "` + outputFile + `"
`
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	artifactsPath := "/test/artifacts/path"
	stdout, _ := os.Open(os.DevNull)
	stderr, _ := os.Open(os.DevNull)
	defer stdout.Close()
	defer stderr.Close()

	err := RunAnalysisWithIO(scriptPath, artifactsPath, stdout, stderr)
	if err != nil {
		t.Fatalf("RunAnalysisWithIO() error = %v", err)
	}

	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	got := strings.TrimRight(string(output), "\n")
	if got != artifactsPath {
		t.Errorf("script received arg = %q, want %q", got, artifactsPath)
	}
}

