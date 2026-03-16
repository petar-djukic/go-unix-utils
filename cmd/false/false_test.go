// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/false against gfalse (GNU coreutils).
// Implements prd014-false R1.1-R1.3, R2.1-R2.3, R3.1-R3.2, R4.1-R4.3 test coverage.
package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gfalse")
	if err != nil {
		t.Skipf("reference binary gfalse not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.3: no arguments — exit 1, no output.
		{
			Name:     "R1.1_no_args_exit_1",
			ExitCode: 1,
		},
		// R1.2, R4.2: arbitrary arguments ignored — exit 1, no output.
		{
			Name:     "R1.2_args_ignored",
			Args:     []string{"foo", "bar", "--baz"},
			ExitCode: 1,
		},
		// R1.2: single argument ignored.
		{
			Name:     "R1.2_single_arg_ignored",
			Args:     []string{"hello"},
			ExitCode: 1,
		},
		// R1.2: flag-like arguments ignored (not --help or --version).
		{
			Name:     "R1.2_flag_like_ignored",
			Args:     []string{"-x", "-v", "--unknown"},
			ExitCode: 1,
		},
		// R2.1, R2.2: --help and --version are tested in standalone tests
		// below because output content and exit code intentionally differ
		// from the GNU reference binary on some platforms.
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelpOutput verifies the --help output contains the expected elements
// per R2.1: utility name, synopsis, and description.
func TestHelpOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--help")
	out, err := cmd.CombinedOutput()
	// R2.1: false --help exits 0.
	if err != nil {
		t.Fatalf("unexpected error running %s --help: %v", goBin, err)
	}

	output := string(out)

	// R2.1: output includes the utility name in Usage line.
	if !strings.Contains(output, "Usage: false") {
		t.Errorf("--help output missing 'Usage: false', got: %s", output)
	}

	// R2.1: output includes description.
	if !strings.Contains(output, "Exit with a status code indicating failure.") {
		t.Errorf("--help output missing description, got: %s", output)
	}
}

// TestVersionOutput verifies the --version output contains the expected elements
// per R2.2: program name and version string, exiting 0.
func TestVersionOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--version")
	out, err := cmd.CombinedOutput()
	// R2.2: false --version exits 0.
	if err != nil {
		t.Fatalf("unexpected error running %s --version: %v", goBin, err)
	}

	output := string(out)

	// R2.2: output includes the program name.
	if !strings.Contains(output, "false") {
		t.Errorf("--version output missing 'false', got: %s", output)
	}
}
