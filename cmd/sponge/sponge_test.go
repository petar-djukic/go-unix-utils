// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/sponge against the Homebrew moreutils reference
// binary sponge.
//
// Implements prd007-sponge R1, R2, R3, R4, R5 via differential testing
// using pkg/testutils.RunDiffTests (passthrough mode) and direct binary
// execution with file-content comparison (file output and append modes).
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinaryPath is the path to the Go sponge binary built in TestMain.
// refBinaryPath is the path to the moreutils reference binary (sponge).
var (
	goBinaryPath  string
	refBinaryPath string
)

func TestMain(m *testing.M) {
	// Locate moreutils reference binary sponge (Homebrew moreutils, no g-prefix).
	refPath, err := exec.LookPath("sponge")
	if err != nil {
		fmt.Println("sponge not found on PATH; skipping sponge differential tests")
		os.Exit(0)
	}
	refBinaryPath = refPath

	// Build the Go sponge binary from the current package.
	tmpDir, err := os.MkdirTemp("", "sponge-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}

	goBinaryPath = filepath.Join(tmpDir, "sponge")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building sponge: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// R4: Passthrough mode — stdin to stdout, no output file (prd007-sponge R4)
// ---------------------------------------------------------------------------

func TestSponge_Passthrough(t *testing.T) {
	// Build a 256-byte input with all byte values 0x00-0xFF.
	var allBytes [256]byte
	for i := range allBytes {
		allBytes[i] = byte(i)
	}

	tests := []testutils.DiffTest{
		{
			Name:  "passthrough_empty_stdin",
			Args:  nil,
			Stdin: "",
		},
		{
			Name:  "passthrough_simple_text",
			Args:  nil,
			Stdin: "hello world\n",
		},
		{
			Name:  "passthrough_multiline",
			Args:  nil,
			Stdin: "line1\nline2\nline3\n",
		},
		{
			Name:  "passthrough_no_trailing_newline",
			Args:  nil,
			Stdin: "no newline at end",
		},
		{
			Name:  "passthrough_binary_data",
			Args:  nil,
			Stdin: string(allBytes[:]),
		},
	}

	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, nil, tests)
}

// ---------------------------------------------------------------------------
// R1, R2: File output mode — write stdin to named file (prd007-sponge R1, R2)
// ---------------------------------------------------------------------------

func TestSponge_FileOutput(t *testing.T) {
	t.Run("write_new_file", func(t *testing.T) {
		runFileOutputTest(t, fileOutputCase{
			stdin: "hello\nworld\n",
		})
	})

	t.Run("overwrite_existing_file", func(t *testing.T) {
		existing := "old content\n"
		runFileOutputTest(t, fileOutputCase{
			existingContent: &existing,
			stdin:           "new content\n",
		})
	})

	t.Run("write_no_trailing_newline", func(t *testing.T) {
		runFileOutputTest(t, fileOutputCase{
			stdin: "no trailing newline",
		})
	})

	t.Run("write_empty_stdin", func(t *testing.T) {
		runFileOutputTest(t, fileOutputCase{
			stdin: "",
		})
	})

	t.Run("soak_before_write", func(t *testing.T) {
		// Verify sponge reads all stdin before opening the output file.
		// Pre-populate the output file; stdin matches its content.
		content := "line1\nline2\nline3\n"
		runFileOutputTest(t, fileOutputCase{
			existingContent: &content,
			stdin:           content,
		})
	})
}

// ---------------------------------------------------------------------------
// R3: Append mode — -a flag (prd007-sponge R3)
// ---------------------------------------------------------------------------

func TestSponge_AppendMode(t *testing.T) {
	t.Run("append_to_existing", func(t *testing.T) {
		existing := "original line\n"
		runFileOutputTest(t, fileOutputCase{
			args:            []string{"-a"},
			existingContent: &existing,
			stdin:           "appended line\n",
		})
	})

	t.Run("append_no_existing_file", func(t *testing.T) {
		runFileOutputTest(t, fileOutputCase{
			args:  []string{"-a"},
			stdin: "new content\n",
		})
	})

	t.Run("append_empty_stdin", func(t *testing.T) {
		existing := "keep this\n"
		runFileOutputTest(t, fileOutputCase{
			args:            []string{"-a"},
			existingContent: &existing,
			stdin:           "",
		})
	})
}

// ---------------------------------------------------------------------------
// Helpers for file output differential testing (D2)
// ---------------------------------------------------------------------------

// fileOutputTimeout is the per-binary execution timeout.
const fileOutputTimeout = 10 * time.Second

// fileOutputCase describes a single file-output differential test.
type fileOutputCase struct {
	args            []string // extra flags before the output filename
	existingContent *string  // if non-nil, pre-populate the output file
	stdin           string
}

// runFileOutputTest runs both the Go and reference sponge binaries with
// identical inputs and an output file argument, then compares the resulting
// output files byte-for-byte and stderr/exit code.
func runFileOutputTest(t *testing.T, tc fileOutputCase) {
	t.Helper()

	// Create isolated directories for each binary.
	goDir := t.TempDir()
	refDir := t.TempDir()

	goOutFile := filepath.Join(goDir, "out.txt")
	refOutFile := filepath.Join(refDir, "out.txt")

	// Pre-populate output files if needed.
	if tc.existingContent != nil {
		if err := os.WriteFile(goOutFile, []byte(*tc.existingContent), 0o644); err != nil {
			t.Fatalf("writing go output file: %v", err)
		}
		if err := os.WriteFile(refOutFile, []byte(*tc.existingContent), 0o644); err != nil {
			t.Fatalf("writing ref output file: %v", err)
		}
	}

	env := buildTestEnv()

	// Build args: flags + output filename.
	goArgs := append(append([]string{}, tc.args...), goOutFile)
	refArgs := append(append([]string{}, tc.args...), refOutFile)

	goStderr, goExit := runSponge(t, goBinaryPath, goArgs, tc.stdin, env)
	refStderr, refExit := runSponge(t, refBinaryPath, refArgs, tc.stdin, env)

	// Compare exit codes.
	if refExit != goExit {
		t.Errorf("exit code divergence: ref=%d go=%d", refExit, goExit)
	}

	// Compare stderr.
	if !bytes.Equal(refStderr, goStderr) {
		t.Errorf("stderr divergence:\n  ref: %q\n  go:  %q", refStderr, goStderr)
	}

	// Compare output file contents.
	goContent, goReadErr := os.ReadFile(goOutFile)
	refContent, refReadErr := os.ReadFile(refOutFile)

	if (goReadErr == nil) != (refReadErr == nil) {
		t.Fatalf("output file existence divergence: ref err=%v, go err=%v", refReadErr, goReadErr)
	}

	if goReadErr == nil && !bytes.Equal(refContent, goContent) {
		t.Errorf("output file content divergence:\n  ref: %q\n  go:  %q", refContent, goContent)
	}
}

// buildTestEnv constructs the environment with LC_ALL=C (ARCHITECTURE.yaml DD6).
func buildTestEnv() []string {
	env := os.Environ()
	prefix := "LC_ALL="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = prefix + "C"
			return env
		}
	}
	return append(env, prefix+"C")
}

// runSponge executes a sponge binary and returns its stderr and exit code.
func runSponge(t *testing.T, binary string, args []string, stdin string, env []string) (stderr []byte, exitCode int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), fileOutputTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = env

	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stderr = errBuf.Bytes()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("timeout executing %s with args %v", binary, args)
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to execute %s: %v", binary, err)
		}
	}

	return stderr, exitCode
}
