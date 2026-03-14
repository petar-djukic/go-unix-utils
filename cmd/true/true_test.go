// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd013-true R4.1–R4.3 (differential tests)
package main

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for true.
const refBinaryName = "gtrue"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []testutils.DiffTest{
		// R1.1: no arguments — exit 0, no output.
		{
			Name: "no_args",
			Args: []string{},
		},
		// R1.2: arbitrary positional arguments — still exit 0, no output.
		{
			Name: "single_arg",
			Args: []string{"foo"},
		},
		{
			Name: "multiple_args",
			Args: []string{"foo", "bar", "baz"},
		},
		// R1.2: flag-like arguments are ignored, not parsed.
		{
			Name: "flag_like_args",
			Args: []string{"--bar", "-x", "--unknown=value"},
		},
		// R1.3: verify no output on stdout or stderr (implicit via diff harness).
		{
			Name: "empty_string_arg",
			Args: []string{""},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffHelpVersion verifies R2.1 and R2.2: --help and --version exit 0.
// Output content differs between implementations, so stdout/stderr are
// normalized to empty; only exit codes are compared.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// --help and --version produce different output between implementations,
	// so we only compare exit codes by normalizing stdout/stderr to empty.
	clearOutput := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		// R2.1: --help exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R2.2: --version exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestNoOutput verifies R4.3: no output is produced on stdout or stderr when
// invoked without --help or --version. This is an explicit check beyond the
// implicit verification provided by the differential harness in TestDiff.
func TestNoOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cases := []struct {
		name string
		args []string
	}{
		{"no_args", nil},
		{"single_arg", []string{"foo"}},
		{"multiple_args", []string{"foo", "bar", "baz"}},
		{"flag_like_args", []string{"--bar", "-x", "--unknown=value"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := exec.Command(goBin, tc.args...)
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			if err != nil {
				t.Fatalf("expected exit 0, got error: %v", err)
			}
			// R4.3: stdout must be empty.
			if stdout.Len() != 0 {
				t.Errorf("expected empty stdout, got %q", stdout.Bytes())
			}
			// R4.3: stderr must be empty.
			if stderr.Len() != 0 {
				t.Errorf("expected empty stderr, got %q", stderr.Bytes())
			}
		})
	}
}

// TestWriteError verifies R2.3: exit 1 when a write error occurs during
// --help or --version output.
func TestWriteError(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	for _, flag := range []string{"--help", "--version"} {
		t.Run(flag, func(t *testing.T) {
			t.Parallel()

			// Create a pipe and close the read end before the binary writes,
			// causing a write error on stdout.
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatalf("creating pipe: %v", err)
			}
			r.Close() // close read end so writes to w fail

			cmd := exec.Command(goBin, flag)
			cmd.Stdout = w
			cmd.Stderr = nil

			// R2.3: the binary should exit non-zero due to write error or SIGPIPE.
			runErr := cmd.Run()
			w.Close() // best-effort cleanup

			if runErr == nil {
				t.Fatalf("expected non-zero exit for %s when stdout is broken, got exit 0", flag)
			}
		})
	}
}
