// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/dirname against gdirname (GNU coreutils).
// Implements prd016-dirname R1.1-R1.5, R2.1-R2.2, R3.1-R3.3 test coverage.
package main

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdirname")
	if err != nil {
		t.Skipf("reference binary gdirname not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path — strip last component.
		{
			Name:     "R1.1_simple_path",
			Args:     []string{"/usr/bin/sort"},
			ExitCode: 0,
		},
		// R1.1: nested path.
		{
			Name:     "R1.1_nested_path",
			Args:     []string{"/a/b/c"},
			ExitCode: 0,
		},
		// R1.1: two-component path.
		{
			Name:     "R1.1_two_component",
			Args:     []string{"dir/file"},
			ExitCode: 0,
		},
		// R1.2 (PRD): no slash — output ".".
		{
			Name:     "R1.2_no_slash",
			Args:     []string{"file.txt"},
			ExitCode: 0,
		},
		// R1.2 (PRD): dot path.
		{
			Name:     "R1.2_dot_path",
			Args:     []string{"."},
			ExitCode: 0,
		},
		// R1.2 (PRD): double-dot path.
		{
			Name:     "R1.2_dotdot_path",
			Args:     []string{".."},
			ExitCode: 0,
		},
		// R1.3 (PRD): root path.
		{
			Name:     "R1.3_root_path",
			Args:     []string{"/"},
			ExitCode: 0,
		},
		// R1.3 (PRD): multiple slashes.
		{
			Name:     "R1.3_multiple_slashes",
			Args:     []string{"///"},
			ExitCode: 0,
		},
		// R1.1 + R1.4 (PRD): trailing slashes stripped.
		{
			Name:     "R1.1_trailing_slashes",
			Args:     []string{"/usr/bin/"},
			ExitCode: 0,
		},
		// Trailing slashes on deeper path.
		{
			Name:     "trailing_slashes_deep",
			Args:     []string{"/usr/bin///"},
			ExitCode: 0,
		},
		// R1.1: root-relative path ("/foo" → "/").
		{
			Name:     "R1.1_root_relative",
			Args:     []string{"/foo"},
			ExitCode: 0,
		},
		// Multiple arguments — dirname accepts them without -a.
		{
			Name:     "multiple_args",
			Args:     []string{"/usr/bin", "/a/b/c"},
			ExitCode: 0,
		},
		// Empty string argument.
		{
			Name:     "empty_string",
			Args:     []string{""},
			ExitCode: 0,
		},
		// No arguments — exit 1.
		{
			Name:      "no_args_error",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// -z NUL-delimited output.
		{
			Name:     "zero_terminator",
			Args:     []string{"-z", "/usr/bin/sort"},
			ExitCode: 0,
		},
		// -z with multiple args.
		{
			Name:     "zero_multiple",
			Args:     []string{"-z", "/usr/bin", "/a/b"},
			ExitCode: 0,
		},
		// Invalid option — exit 1.
		{
			Name:      "invalid_option",
			Args:      []string{"--invalid-option"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// --help exits 0.
		{
			Name:      "help_exits_0",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
		// --version exits 0.
		{
			Name:      "version_exits_0",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelpOutput verifies the --help output contains expected elements.
func TestHelpOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error running %s --help: %v", goBin, err)
	}

	output := string(out)

	if !strings.Contains(output, "Usage: dirname") {
		t.Errorf("--help output missing 'Usage: dirname', got: %s", output)
	}

	if !strings.Contains(output, "--zero") {
		t.Errorf("--help output missing '--zero', got: %s", output)
	}
}

// TestVersionOutput verifies the --version output contains the program name.
func TestVersionOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error running %s --version: %v", goBin, err)
	}

	output := string(out)

	if !strings.Contains(output, "dirname") {
		t.Errorf("--version output missing 'dirname', got: %s", output)
	}
}

// TestNoArgsError verifies R3.2: no arguments produces "missing operand"
// on stderr and exits 1.
func TestNoArgsError(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got error: %v", err)
	}

	output := stderr.String()
	if !strings.Contains(output, "missing operand") {
		t.Errorf("stderr should contain 'missing operand', got: %s", output)
	}
	if !strings.Contains(output, "Try 'dirname --help'") {
		t.Errorf("stderr should contain help hint, got: %s", output)
	}
}

// TestWriteErrorExitCode verifies R3.3: dirname exits non-zero when stdout
// encounters a write error. We generate many arguments and immediately close
// the stdout pipe to trigger the error. The process may exit via SIGPIPE
// handler or via the write error check; either produces a non-zero exit.
func TestWriteErrorExitCode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Generate many arguments to ensure writes occur after pipe close.
	args := make([]string, 4096)
	for i := range args {
		args[i] = "/usr/bin/sort"
	}

	cmd := exec.Command(goBin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	// Close stdout pipe immediately to trigger write error.
	stdout.Close()

	err = cmd.Wait()
	if err == nil {
		t.Skip("write error did not trigger non-zero exit (platform-dependent)")
	}

	// Verify exit code is non-zero (1 from write error or signal-based exit).
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.ExitCode() == 0 {
			t.Errorf("expected non-zero exit code on write error, got 0")
		}
	}
	// If err is not an ExitError, the process was killed by a signal (SIGPIPE),
	// which is also acceptable behavior for R3.3.
}

// normalizeAllOutput replaces all output with empty bytes so that only
// exit codes are compared. Used for --help/--version where output content
// intentionally differs between implementations.
func normalizeAllOutput(b []byte) []byte {
	return nil
}

// normalizeStderr replaces all output with empty bytes for error message
// comparison where the exact message format may differ between implementations.
func normalizeStderr(b []byte) []byte {
	return nil
}
