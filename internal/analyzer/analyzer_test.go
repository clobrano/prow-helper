package analyzer

import (
	"errors"
	"os"
	"path/filepath"
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
	err := RunAnalysis("", "/some/path")
	if err != nil {
		t.Errorf("RunAnalysis() with empty command should not error, got %v", err)
	}
}

func TestRunAnalysis_WhitespaceCommand(t *testing.T) {
	err := RunAnalysis("   ", "/some/path")
	if err != nil {
		t.Errorf("RunAnalysis() with whitespace command should not error, got %v", err)
	}
}

func TestRunAnalysis_SuccessfulCommand(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a simple command that should succeed and accept an argument
	err := RunAnalysis("ls", tmpDir)
	if err != nil {
		t.Errorf("RunAnalysis() error = %v, want nil", err)
	}
}

func TestRunAnalysis_FailingCommand(t *testing.T) {
	// Use a command that will fail
	err := RunAnalysis("false", "/some/path")
	if err == nil {
		t.Error("RunAnalysis() should return error for failing command")
		return
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Errorf("RunAnalysis() error should be ExitError, got %T", err)
		return
	}

	if exitErr.ExitCode == 0 {
		t.Error("ExitError.ExitCode should be non-zero for failed command")
	}
}

func TestRunAnalysis_NonExistentCommand(t *testing.T) {
	err := RunAnalysis("nonexistent-command-12345", "/some/path")
	if err == nil {
		t.Error("RunAnalysis() should return error for non-existent command")
	}
}

func TestRunAnalysis_PassesArtifactsPath(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	// Create a script that writes its argument to a file
	scriptContent := `#!/bin/bash
echo "$1" > "` + outputFile + `"
`
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}

	artifactsPath := "/test/artifacts/path"
	err := RunAnalysis(scriptPath, artifactsPath)
	if err != nil {
		t.Fatalf("RunAnalysis() error = %v", err)
	}

	// Check that the artifacts path was passed as argument
	output, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	// Trim newline
	got := string(output)
	got = got[:len(got)-1] // Remove trailing newline

	if got != artifactsPath {
		t.Errorf("Script received argument = %v, want %v", got, artifactsPath)
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
