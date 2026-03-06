// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tee against the GNU reference binary (gtee).
// Implements prd017-tee R4: differential testing.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildBinary compiles cmd/tee and returns the path to the built binary.
func buildBinary(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "tee")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Dir(testFilePath())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build tee binary: %v\n%s", err, out)
	}
	return bin
}

// testFilePath returns the directory containing this test file.
func testFilePath() string {
	// Use runtime caller or just return "." since tests run in the package dir.
	return "."
}

// runResult captures the output of a binary execution.
type runResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBinary executes a binary with the given args, stdin, env, and working directory.
func runBinary(t *testing.T, bin string, args []string, stdin []byte, env []string, workDir string) runResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Dir = workDir

	baseEnv := os.Environ()
	cmd.Env = append(baseEnv, env...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("failed to run %s: %v", bin, err)
	}

	return runResult{
		stdout:   stdoutBuf.Bytes(),
		stderr:   stderrBuf.Bytes(),
		exitCode: exitCode,
	}
}

// diffTest defines a single differential test case.
type diffTest struct {
	name     string
	args     []string
	stdin    []byte
	env      []string
	exitCode int
	// setupFiles maps relative paths to content that should be pre-created before each binary run.
	setupFiles map[string][]byte
	// checkFiles lists relative paths whose contents should match between go and ref binary runs.
	checkFiles []string
}

func TestDiff(t *testing.T) {
	goBin := buildBinary(t)
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	tests := []diffTest{
		// R1: passthrough — no file arguments, stdin to stdout only.
		{
			name:     "passthrough",
			args:     []string{},
			stdin:    []byte("hello\nworld\n"),
			env:      []string{"LC_ALL=C"},
			exitCode: 0,
		},
		// R1: empty stdin — creates empty output file, no stdout.
		{
			name:       "empty_stdin_with_file",
			args:       []string{"empty.txt"},
			stdin:      []byte(""),
			env:        []string{"LC_ALL=C"},
			exitCode:   0,
			checkFiles: []string{"empty.txt"},
		},
		// R1: single file output.
		{
			name:       "single_file",
			args:       []string{"out.txt"},
			stdin:      []byte("hello\nworld\n"),
			env:        []string{"LC_ALL=C"},
			exitCode:   0,
			checkFiles: []string{"out.txt"},
		},
		// R1: multiple file output.
		{
			name:       "multiple_files",
			args:       []string{"a.txt", "b.txt"},
			stdin:      []byte("line1\nline2\n"),
			env:        []string{"LC_ALL=C"},
			exitCode:   0,
			checkFiles: []string{"a.txt", "b.txt"},
		},
		// R1.4: dash as stdout reference.
		{
			name:     "dash_stdout",
			args:     []string{"-"},
			stdin:    []byte("data\n"),
			env:      []string{"LC_ALL=C"},
			exitCode: 0,
		},
		// R2: append mode — appends to existing file.
		{
			name:       "append_mode",
			args:       []string{"-a", "existing.txt"},
			stdin:      []byte("new\n"),
			env:        []string{"LC_ALL=C"},
			exitCode:   0,
			setupFiles: map[string][]byte{"existing.txt": []byte("old\n")},
			checkFiles: []string{"existing.txt"},
		},
		// R1.3: truncate by default — overwrites existing file.
		{
			name:       "truncate_default",
			args:       []string{"existing.txt"},
			stdin:      []byte("replacement\n"),
			env:        []string{"LC_ALL=C"},
			exitCode:   0,
			setupFiles: map[string][]byte{"existing.txt": []byte("original content\n")},
			checkFiles: []string{"existing.txt"},
		},
		// R3: write error — nonexistent directory.
		{
			name:     "write_error_nonexistent_dir",
			args:     []string{"/nonexistent-dir/file.txt"},
			stdin:    []byte("data\n"),
			env:      []string{"LC_ALL=C"},
			exitCode: 1,
		},
		// R3.3: write error with good file — continues writing to other destinations.
		{
			name:       "write_error_with_good_file",
			args:       []string{"/nonexistent-dir/file.txt", "good.txt"},
			stdin:      []byte("data\n"),
			env:        []string{"LC_ALL=C"},
			exitCode:   1,
			checkFiles: []string{"good.txt"},
		},
		// R1.5: binary data passthrough.
		{
			name:       "binary_passthrough",
			args:       []string{"binary.out"},
			stdin:      []byte{0x00, 0x01, 0xff, 0x0a},
			env:        []string{"LC_ALL=C"},
			exitCode:   0,
			checkFiles: []string{"binary.out"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create separate work directories for each binary.
			goDir := t.TempDir()
			refDir := t.TempDir()

			// Pre-create setup files in both directories.
			for name, content := range tc.setupFiles {
				writeTestFile(t, filepath.Join(goDir, name), content)
				writeTestFile(t, filepath.Join(refDir, name), content)
			}

			// Resolve file args to absolute paths within each work dir.
			goArgs := resolveArgs(tc.args, goDir)
			refArgs := resolveArgs(tc.args, refDir)

			goResult := runBinary(t, goBin, goArgs, tc.stdin, tc.env, goDir)
			refResult := runBinary(t, refBin, refArgs, tc.stdin, tc.env, refDir)

			// Compare exit codes.
			if goResult.exitCode != refResult.exitCode {
				t.Errorf("exit code mismatch: go=%d ref=%d", goResult.exitCode, refResult.exitCode)
			}

			// Compare stdout.
			if !bytes.Equal(goResult.stdout, refResult.stdout) {
				t.Errorf("stdout mismatch:\n  go:  %q\n  ref: %q", goResult.stdout, refResult.stdout)
			}

			// Compare stderr presence (both should have error or both should be clean).
			goHasErr := len(goResult.stderr) > 0
			refHasErr := len(refResult.stderr) > 0
			if goHasErr != refHasErr {
				t.Errorf("stderr presence mismatch:\n  go:  %q\n  ref: %q", goResult.stderr, refResult.stderr)
			}

			// Compare output file contents.
			for _, name := range tc.checkFiles {
				goContent := readTestFile(t, filepath.Join(goDir, name))
				refContent := readTestFile(t, filepath.Join(refDir, name))
				if !bytes.Equal(goContent, refContent) {
					t.Errorf("file %s mismatch:\n  go:  %q\n  ref: %q", name, goContent, refContent)
				}
			}
		})
	}
}

// resolveArgs replaces relative file path arguments with absolute paths under dir.
// Arguments starting with "-" or "/" are left unchanged.
func resolveArgs(args []string, dir string) []string {
	resolved := make([]string, len(args))
	for i, arg := range args {
		if arg == "-" || len(arg) == 0 {
			resolved[i] = arg
		} else if arg[0] == '-' || arg[0] == '/' {
			resolved[i] = arg
		} else {
			resolved[i] = filepath.Join(dir, arg)
		}
	}
	return resolved
}

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create directory for %s: %v", path, err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

// readTestFile reads a file's contents, failing the test if the file cannot be read.
func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return content
}
