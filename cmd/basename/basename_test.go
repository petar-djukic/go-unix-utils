// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/basename against gbasename (GNU coreutils).
// Implements prd015-basename R1.1-R1.5, R2.1-R2.3, R3.1-R3.4, R4.1-R4.3 test coverage.
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
	refBin, err := exec.LookPath("gbasename")
	if err != nil {
		t.Skipf("reference binary gbasename not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: simple path — strip directory prefix.
		{
			Name:     "R1.1_simple_path",
			Args:     []string{"/usr/bin/sort"},
			ExitCode: 0,
		},
		// R1.2: suffix removal with two operands.
		{
			Name:     "R1.2_suffix_removal",
			Args:     []string{"include/stdio.h", ".h"},
			ExitCode: 0,
		},
		// R1.3: trailing slashes stripped.
		{
			Name:     "R1.3_trailing_slashes",
			Args:     []string{"/usr/bin/sort///"},
			ExitCode: 0,
		},
		// R1.4: all-slash input returns "/".
		{
			Name:     "R1.4_root_path",
			Args:     []string{"/"},
			ExitCode: 0,
		},
		// R1.4: multiple slashes — still returns "/".
		{
			Name:     "R1.4_multiple_slashes",
			Args:     []string{"///"},
			ExitCode: 0,
		},
		// R1.5: empty string input.
		{
			Name:     "R1.5_empty_string",
			Args:     []string{""},
			ExitCode: 0,
		},
		// R1.2: suffix equals result — no removal.
		{
			Name:     "R1.2_suffix_equals_result",
			Args:     []string{"bar", "bar"},
			ExitCode: 0,
		},
		// R1.1: no directory — returns name unchanged.
		{
			Name:     "R1.1_no_directory",
			Args:     []string{"hello"},
			ExitCode: 0,
		},
		// R2.1: -a multiple argument mode.
		{
			Name:     "R2.1_multiple_mode",
			Args:     []string{"-a", "/usr/bin/sort", "/usr/bin/cat"},
			ExitCode: 0,
		},
		// R2.2: -s suffix with multiple names.
		{
			Name:     "R2.2_suffix_option",
			Args:     []string{"-s", ".h", "include/stdio.h", "include/stdlib.h"},
			ExitCode: 0,
		},
		// R2.3: -a without suffix — no suffix removal.
		{
			Name:     "R2.3_multiple_no_suffix",
			Args:     []string{"-a", "file.txt", "other.txt"},
			ExitCode: 0,
		},
		// R3.1: -z NUL-delimited output.
		{
			Name:     "R3.1_zero_terminator",
			Args:     []string{"-z", "hello"},
			ExitCode: 0,
		},
		// R3.1: -z with -a multiple mode.
		{
			Name:     "R3.1_zero_with_multiple",
			Args:     []string{"-az", "/usr/bin/sort", "/usr/bin/cat"},
			ExitCode: 0,
		},
		// R2.2: --suffix=SUFFIX long form.
		{
			Name:     "R2.2_long_suffix",
			Args:     []string{"--suffix=.txt", "file.txt", "other.txt"},
			ExitCode: 0,
		},
		// R2.1: --multiple long form.
		{
			Name:     "R2.1_long_multiple",
			Args:     []string{"--multiple", "/a/b", "/c/d"},
			ExitCode: 0,
		},
		// R3.1: --zero long form.
		{
			Name:     "R3.1_long_zero",
			Args:     []string{"--zero", "hello"},
			ExitCode: 0,
		},
		// R3.3: no arguments — exit 1.
		{
			Name:     "R3.3_no_args_error",
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// R3.3: extra operand in single mode — exit 1.
		{
			Name:     "R3.3_extra_operand_error",
			Args:     []string{"a", "b", "c"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{normalizeStderr},
		},
		// Combined: -s with -z.
		{
			Name:     "combined_suffix_zero",
			Args:     []string{"-s", ".h", "-z", "stdio.h", "stdlib.h"},
			ExitCode: 0,
		},
		// R2.3 (task R4): --help exits 0.
		{
			Name:      "R2.3_help_exits_0",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
		// R2.3 (task R4): --version exits 0.
		{
			Name:      "R2.3_version_exits_0",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelpOutput verifies the --help output contains the expected elements.
func TestHelpOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("unexpected error running %s --help: %v", goBin, err)
	}

	output := string(out)

	if !strings.Contains(output, "Usage: basename") {
		t.Errorf("--help output missing 'Usage: basename', got: %s", output)
	}

	if !strings.Contains(output, "--multiple") {
		t.Errorf("--help output missing '--multiple', got: %s", output)
	}

	if !strings.Contains(output, "--suffix") {
		t.Errorf("--help output missing '--suffix', got: %s", output)
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

	if !strings.Contains(output, "basename") {
		t.Errorf("--version output missing 'basename', got: %s", output)
	}
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
