// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd016-dirname R4.1–R4.3 (differential tests for R1.1–R1.5, R2.1, R2.2, R3.1–R3.3)
package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for dirname.
const refBinaryName = "gdirname"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path — strip last component.
		{
			Name: "simple_path",
			Args: []string{"/usr/bin/sort"},
		},
		// R1.1: nested path.
		{
			Name: "nested_path",
			Args: []string{"/a/b/c"},
		},
		// R1.1: relative path with directory.
		{
			Name: "relative_path",
			Args: []string{"dir/file"},
		},
		// R1.2: no slash — output dot.
		{
			Name: "no_slash",
			Args: []string{"file.txt"},
		},
		// R1.2: dot path.
		{
			Name: "dot_path",
			Args: []string{"."},
		},
		// R1.2: double-dot path.
		{
			Name: "double_dot_path",
			Args: []string{".."},
		},
		// R1.3: trailing slashes stripped before extraction.
		{
			Name: "trailing_slashes",
			Args: []string{"/usr/bin/"},
		},
		// R1.3: multiple trailing slashes.
		{
			Name: "multiple_trailing_slashes",
			Args: []string{"/usr/bin///"},
		},
		// R1.4: root path.
		{
			Name: "root_path",
			Args: []string{"/"},
		},
		// R1.4: multiple slashes.
		{
			Name: "all_slashes",
			Args: []string{"///"},
		},
		// R1.1: deep path.
		{
			Name: "deep_path",
			Args: []string{"/a/b/c/d/e/file.txt"},
		},
		// R1.1: path with single leading component.
		{
			Name: "single_leading_slash_component",
			Args: []string{"/usr"},
		},
		// R1.5: multiple arguments.
		{
			Name: "multiple_args",
			Args: []string{"/usr/bin/sort", "/usr/bin/cat"},
		},
		// R1.5: multiple args with mixed cases.
		{
			Name: "multiple_mixed",
			Args: []string{"file.txt", "/usr/bin/", "/", "a/b/c"},
		},
		// R2.1: -z flag — NUL-terminated output, single arg.
		{
			Name: "zero_single",
			Args: []string{"-z", "/usr/bin"},
		},
		// R2.1: --zero long flag.
		{
			Name: "zero_long_flag",
			Args: []string{"--zero", "/usr/bin"},
		},
		// R2.1, R2.2: -z with multiple arguments.
		{
			Name: "zero_multiple",
			Args: []string{"-z", "/usr/bin", "/etc/hosts"},
		},
		// R2.1: -z with mixed edge cases.
		{
			Name: "zero_mixed",
			Args: []string{"-z", "file.txt", "/usr/bin/", "/", "a/b/c"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Error messages differ between implementations; normalize stderr.
	clearOutput := func(b []byte) []byte { return nil }

	// R4.3, R3.2: no arguments → exit 1 with error message to stderr.
	tests := []testutils.DiffTest{
		{
			Name:      "no_args",
			Args:      []string{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffExitZero verifies R3.1: exit 0 on success.
func TestDiffExitZero(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// R3.1: successful invocations exit 0.
	tests := []testutils.DiffTest{
		{
			Name:     "exit_zero_simple",
			Args:     []string{"/usr/bin/sort"},
			ExitCode: 0,
		},
		{
			Name:     "exit_zero_dot",
			Args:     []string{"file.txt"},
			ExitCode: 0,
		},
		{
			Name:     "exit_zero_multiple",
			Args:     []string{"/usr/bin", "/etc"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestWriteError verifies R3.3: exit 1 when a write error occurs on stdout.
func TestWriteError(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Create a pipe and close the read end before the binary writes,
	// causing a write error on stdout.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	r.Close() // close read end so writes to w fail

	cmd := exec.Command(goBin, "/usr/bin/sort")
	cmd.Stdout = w
	cmd.Stderr = nil

	// R3.3: the binary should exit non-zero due to write error or SIGPIPE.
	runErr := cmd.Run()
	w.Close() // best-effort cleanup

	if runErr == nil {
		t.Fatal("expected non-zero exit when stdout is broken, got exit 0")
	}
}
