// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/true against gtrue (GNU coreutils).
// Implements prd013-true R1.1-R1.3, R2.1-R2.3, R3.1-R3.2, R4.1-R4.3 test coverage.
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
	refBin, err := exec.LookPath("gtrue")
	if err != nil {
		t.Skipf("reference binary gtrue not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1, R4.3: no arguments — exit 0, no output.
		{
			Name:     "R1.1_no_args_exit_0",
			ExitCode: 0,
		},
		// R1.2, R4.2: arbitrary arguments ignored — exit 0, no output.
		{
			Name:     "R1.2_args_ignored",
			Args:     []string{"foo", "bar", "--baz"},
			ExitCode: 0,
		},
		// R1.2: single argument ignored.
		{
			Name:     "R1.2_single_arg_ignored",
			Args:     []string{"hello"},
			ExitCode: 0,
		},
		// R1.2: flag-like arguments ignored (not --help or --version).
		{
			Name:     "R1.2_flag_like_ignored",
			Args:     []string{"-x", "-v", "--unknown"},
			ExitCode: 0,
		},
		// R2.1, R2.2, R4.2: --help prints usage and exits 0.
		{
			Name:      "R2.1_help_exits_0",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeHelpOutput},
		},
		// R2.2, R4.2: --version prints version info and exits 0.
		{
			Name:      "R2.2_version_exits_0",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelpOutput verifies the --help output contains the expected elements
// per R2.2 and R2.3: utility name, synopsis, and description.
func TestHelpOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error running %s --help: %v", goBin, err)
	}

	output := string(out)

	// R2.2: output includes the utility name in Usage line.
	if !strings.Contains(output, "Usage: true") {
		t.Errorf("--help output missing 'Usage: true', got: %s", output)
	}

	// R2.3: output includes synopsis and description.
	if !strings.Contains(output, "Exit with a status code indicating success.") {
		t.Errorf("--help output missing description, got: %s", output)
	}
}

// TestWriteErrorExitsZero verifies R3.1: true exits 0 even when stdout
// write fails. We redirect stdout to /dev/full on Linux or simulate by
// closing stdout via a subprocess that closes fd 1.
func TestWriteErrorExitsZero(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Run true --help with stdout closed (write will fail).
	// Use shell to close fd 1 before exec.
	cmd := exec.Command("sh", "-c", goBin+" --help >&- 2>/dev/null")
	err := cmd.Run()

	// R3.1: must exit 0 regardless of write errors.
	if err != nil {
		t.Errorf("expected exit 0 on stdout write error, got: %v", err)
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
