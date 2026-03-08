// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge against the sponge reference binary.
//
// Implements prd007-sponge acceptance criteria AC1-AC5 via testutils.RunDiffTests
// for passthrough mode and custom file-content comparison for file-output mode.
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
	refBin, err := exec.LookPath("sponge")
	if err != nil {
		t.Skipf("reference binary sponge not in PATH: %v", err)
	}

	// Passthrough mode tests: no file argument, stdout comparison via RunDiffTests.
	stdoutTests := []testutils.DiffTest{
		// R4.1: No filename, stdin appears on stdout.
		{
			Name:  "sponge_passthrough_stdout",
			Stdin: []byte("hello world\n"),
		},
		// R4.1: Empty stdin passthrough.
		{
			Name:  "sponge_empty_passthrough",
			Stdin: []byte{},
		},
		// R4.1: Multi-line passthrough.
		{
			Name:  "sponge_passthrough_multiline",
			Stdin: []byte("line1\nline2\nline3\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, stdoutTests)

	// File-output tests: compare output file content between Go and ref binaries.
	t.Run("sponge_small_stdin_to_file", func(t *testing.T) {
		stdin := []byte("1\n2\n3\n4\n5\n")
		runFileTest(t, goBin, refBin, []string{"outfile.txt"}, stdin, nil)
	})

	t.Run("sponge_empty_stdin_to_file", func(t *testing.T) {
		runFileTest(t, goBin, refBin, []string{"empty.txt"}, []byte{}, nil)
	})

	t.Run("sponge_append_mode", func(t *testing.T) {
		setup := func(dir string) {
			writeFixture(t, dir, "existing.txt", "original line\n")
		}
		runFileTest(t, goBin, refBin, []string{"-a", "existing.txt"}, []byte("appended line\n"), setup)
	})

	t.Run("sponge_append_no_existing_file", func(t *testing.T) {
		runFileTest(t, goBin, refBin, []string{"-a", "newfile.txt"}, []byte("new content\n"), nil)
	})

	t.Run("sponge_soak_before_write", func(t *testing.T) {
		content := "line1\nline2\nline3\n"
		setup := func(dir string) {
			writeFixture(t, dir, "data.txt", content)
		}
		runFileTest(t, goBin, refBin, []string{"data.txt"}, []byte(content), setup)
	})

	t.Run("sponge_large_stdin", func(t *testing.T) {
		// Generate >1MB of input.
		var buf bytes.Buffer
		line := strings.Repeat("abcdefghij", 10) + "\n" // 101 bytes
		for buf.Len() < 1048576 {
			buf.WriteString(line)
		}
		runFileTest(t, goBin, refBin, []string{"large_out.txt"}, buf.Bytes(), nil)
	})
}

// runFileTest runs both binaries in separate temp dirs and compares the output file.
func runFileTest(t *testing.T, goBin, refBin string, args []string, stdin []byte, setup func(dir string)) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()

	if setup != nil {
		setup(refDir)
		setup(goDir)
	}

	env := buildTestEnv()

	refStdout, refStderr, refExit := runBin(t, refBin, args, stdin, env, refDir)
	goStdout, goStderr, goExit := runBin(t, goBin, args, stdin, env, goDir)

	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d\nref stderr: %q\ngo stderr: %q",
			refExit, goExit, refStderr, goStderr)
	}

	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout mismatch:\nref: %q\ngo:  %q", refStdout, goStdout)
	}

	// Compare output files for each arg that looks like a filename (not a flag).
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		refContent := readFileOrEmpty(t, filepath.Join(refDir, arg))
		goContent := readFileOrEmpty(t, filepath.Join(goDir, arg))
		if !bytes.Equal(refContent, goContent) {
			t.Errorf("file %s content mismatch:\nref (%d bytes): %q\ngo  (%d bytes): %q",
				arg, len(refContent), truncate(refContent, 256),
				len(goContent), truncate(goContent, 256))
		}
	}
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

// readFileOrEmpty reads a file, returning empty bytes if it doesn't exist.
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
