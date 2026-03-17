// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/false against gfalse (GNU coreutils).
// Implements prd014-false R4.1-R4.3 test coverage.
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
		// R2.1, R4.2: --help prints usage and exits 1 (GNU false always exits 1).
		{
			Name:      "R2.1_help_exits_1",
			Args:      []string{"--help"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeHelpOutput},
		},
		// R2.2, R4.2: --version prints version info and exits 1 (GNU false always exits 1).
		{
			Name:      "R2.2_version_exits_1",
			Args:      []string{"--version"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelpOutput verifies the --help output contains the expected elements
// per R2.1: utility name, synopsis, and description.
func TestHelpOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--help")
	out, _ := cmd.CombinedOutput()
	// GNU false always exits 1, even for --help.

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

// TestWriteErrorExitsOne verifies R2.3/R3.1: false exits 1 even when stdout
// write fails during --help output.
func TestWriteErrorExitsOne(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Run false --help with stdout closed (write will fail).
	// R3.1: false always exits 1.
	cmd := exec.Command("sh", "-c", goBin+" --help >&- 2>/dev/null")
	err := cmd.Run()

	if err == nil {
		t.Errorf("expected non-zero exit on stdout write error, got exit 0")
	}
}

// normalizeAllOutput replaces all output with empty bytes so that only
// exit codes are compared. Used for --version where output content
// intentionally differs between implementations.
func normalizeAllOutput(b []byte) []byte {
	return nil
}

// normalizeHelpOutput replaces all output with empty bytes so that only
// exit codes are compared. GNU help includes hyperlinks, shell notes, and
// the full binary path which intentionally differ from our implementation.
func normalizeHelpOutput(b []byte) []byte {
	return nil
}
