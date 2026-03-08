// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/tee against the GNU reference binary (gtee).
//
// Implements prd017-tee acceptance criteria AC1-AC5 via testutils.RunDiffTests
// for stdout-only tests and custom file-content comparison for file-writing tests.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtee")
	if err != nil {
		t.Skipf("reference binary gtee not in PATH: %v", err)
	}

	// --- Stdout-only tests via RunDiffTests ---

	stdoutTests := []testutils.DiffTest{
		// R1.2: Passthrough mode — no files, stdin to stdout only.
		{
			Name:  "tee_passthrough",
			Stdin: []byte("hello\nworld\n"),
		},
		// R1.2: Empty stdin passthrough.
		{
			Name:  "tee_empty_stdin_passthrough",
			Stdin: []byte{},
		},
		// R1.4: "-" as file argument — treated as stdout, not duplicated.
		{
			Name:  "tee_dash_stdout",
			Args:  []string{"-"},
			Stdin: []byte("data\n"),
		},
		// R2.2: -i flag accepted without error (signal delivery not automated).
		{
			Name:  "tee_ignore_interrupts_flag",
			Args:  []string{"-i"},
			Stdin: []byte("signal test\n"),
		},
		// R2.3: -a and -i combined, passthrough.
		{
			Name:  "tee_combined_ai_passthrough",
			Args:  []string{"-ai"},
			Stdin: []byte("combined\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, stdoutTests)

	// --- File-writing tests: separate temp dirs for each binary ---

	// R1.1, R1.3: Single file — stdin to stdout and file.
	t.Run("tee_single_file", func(t *testing.T) {
		runFileTest(t, goBin, refBin, nil, []string{"out.txt"}, []byte("hello\nworld\n"), nil)
	})

	// R1.1, R1.5: Multiple files — identical content to both files and stdout.
	t.Run("tee_multiple_files", func(t *testing.T) {
		runFileTest(t, goBin, refBin, nil, []string{"a.txt", "b.txt"}, []byte("line1\nline2\n"), nil)
	})

	// R2.1: Append mode — preserves existing content.
	t.Run("tee_append_mode", func(t *testing.T) {
		setup := func(dir string) {
			writeFixture(t, dir, "existing.txt", "old\n")
		}
		runFileTest(t, goBin, refBin, []string{"-a"}, []string{"existing.txt"}, []byte("new\n"), setup)
	})

	// R1.3: Truncate by default — overwrites existing content.
	t.Run("tee_truncate_default", func(t *testing.T) {
		setup := func(dir string) {
			writeFixture(t, dir, "existing.txt", "original content\n")
		}
		runFileTest(t, goBin, refBin, nil, []string{"existing.txt"}, []byte("replacement\n"), setup)
	})

	// R1.1: Empty stdin creates empty output file.
	t.Run("tee_empty_stdin_file", func(t *testing.T) {
		runFileTest(t, goBin, refBin, nil, []string{"empty.txt"}, []byte{}, nil)
	})

	// R1.5: Binary data passthrough preserves NUL and high bytes.
	t.Run("tee_binary_passthrough", func(t *testing.T) {
		stdin := []byte{0x00, 0x01, 0xff, '\n'}
		runFileTest(t, goBin, refBin, nil, []string{"binary.out"}, stdin, nil)
	})

	// R3.2, R3.3: Write error — bad path fails, good file still written.
	t.Run("tee_write_error", func(t *testing.T) {
		runWriteErrorTest(t, goBin, refBin)
	})
}

// runFileTest runs both binaries in separate temp dirs, compares stdout, exit code,
// and output file contents. flags are CLI flags (e.g. ["-a"]), fileNames are
// relative output file names passed as args after flags.
func runFileTest(t *testing.T, goBin, refBin string, flags, fileNames []string, stdin []byte, setup func(dir string)) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()

	if setup != nil {
		setup(refDir)
		setup(goDir)
	}

	// Build args: flags + absolute file paths in each binary's temp dir.
	refArgs := buildArgs(flags, fileNames, refDir)
	goArgs := buildArgs(flags, fileNames, goDir)

	env := buildTestEnv()

	refStdout, _, refExit := runBin(t, refBin, refArgs, stdin, env, refDir)
	goStdout, _, goExit := runBin(t, goBin, goArgs, stdin, env, goDir)

	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d", refExit, goExit)
	}

	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout mismatch:\nref: %q\ngo:  %q", refStdout, goStdout)
	}

	// Compare output file contents.
	for _, name := range fileNames {
		refContent := readFileOrEmpty(t, filepath.Join(refDir, name))
		goContent := readFileOrEmpty(t, filepath.Join(goDir, name))
		if !bytes.Equal(refContent, goContent) {
			t.Errorf("file %s content mismatch:\nref (%d bytes): %q\ngo  (%d bytes): %q",
				name, len(refContent), truncate(refContent, 256),
				len(goContent), truncate(goContent, 256))
		}
	}
}

// runWriteErrorTest verifies R3.2/R3.3: a bad output path causes exit 1
// but a good output file is still written.
func runWriteErrorTest(t *testing.T, goBin, refBin string) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()

	stdin := []byte("data\n")
	env := buildTestEnv()

	// Bad path that cannot be opened + good file in temp dir.
	badPath := "/nonexistent-dir/file.txt"
	goodRef := filepath.Join(refDir, "good.txt")
	goodGo := filepath.Join(goDir, "good.txt")

	refArgs := []string{badPath, goodRef}
	goArgs := []string{badPath, goodGo}

	refStdout, refStderr, refExit := runBin(t, refBin, refArgs, stdin, env, refDir)
	goStdout, goStderr, goExit := runBin(t, goBin, goArgs, stdin, env, goDir)

	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d", refExit, goExit)
	}

	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout mismatch:\nref: %q\ngo:  %q", refStdout, goStdout)
	}

	// Both should have non-empty stderr.
	if len(refStderr) == 0 {
		t.Error("ref stderr unexpectedly empty")
	}
	if len(goStderr) == 0 {
		t.Error("go stderr unexpectedly empty")
	}

	// Good file should be written by both.
	refContent := readFileOrEmpty(t, goodRef)
	goContent := readFileOrEmpty(t, goodGo)
	if !bytes.Equal(refContent, goContent) {
		t.Errorf("good file content mismatch:\nref: %q\ngo:  %q", refContent, goContent)
	}
}

// buildArgs constructs the argument list from flags and file names resolved
// against a base directory.
func buildArgs(flags, fileNames []string, baseDir string) []string {
	args := make([]string, 0, len(flags)+len(fileNames))
	args = append(args, flags...)
	for _, name := range fileNames {
		args = append(args, filepath.Join(baseDir, name))
	}
	return args
}

// runBin executes a binary and returns stdout, stderr, and exit code.
func runBin(t *testing.T, binary string, args []string, stdin []byte, env []string, dir string) ([]byte, []byte, int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Env = env

	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", binary)
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}

	return stdoutBuf.Bytes(), stderrBuf.Bytes(), exitCode
}

// buildTestEnv returns the process environment with LC_ALL=C set.
func buildTestEnv() []string {
	env := os.Environ()
	for i, entry := range env {
		if strings.HasPrefix(entry, "LC_ALL=") {
			env[i] = "LC_ALL=C"
			return env
		}
	}
	return append(env, "LC_ALL=C")
}

// writeFixture creates a test file with the given content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write fixture %s: %v", name, err)
	}
}

// readFileOrEmpty reads a file, returning nil if it doesn't exist.
func readFileOrEmpty(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}

// truncate returns at most n bytes of data for display purposes.
func truncate(data []byte, n int) []byte {
	if len(data) <= n {
		return data
	}
	return data[:n]
}
