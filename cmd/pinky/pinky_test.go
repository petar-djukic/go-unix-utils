// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/pinky.
// Tests cover srd098-pinky R1 (short format), R2 (long format and field control),
// R3 (error handling, version, help).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrProgRe matches the program name/path prefix before a colon at line start.
var stderrProgRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// stderrTryRe matches the quoted program reference in Try hint lines.
var stderrTryRe = regexp.MustCompile(`'[^']*--help'`)

// idleFieldRe matches the 7-char idle field in pinky short-format output.
// Idle time may differ between two binary invocations because terminal
// device mod times change when a process writes to the terminal.
// Matches: (TTY + spaces)(7-char idle)(month name start of date).
var idleFieldRe = regexp.MustCompile(
	`((?:console|tty\w+)\s+)(.{7})((?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec))`,
)

// stderrNormalizer normalizes program name differences in error messages.
// R3.2: replaces binary paths with "PROG" so error message structure
// can be compared between Go and GNU binaries.
func stderrNormalizer(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	b = stderrProgRe.ReplaceAll(b, []byte("PROG:"))
	b = stderrTryRe.ReplaceAll(b, []byte("'PROG --help'"))
	return b
}

// discardOutput normalizes by discarding all output, used when
// output content differs by design (--version, --help) and only
// exit code comparison is meaningful.
func discardOutput(b []byte) []byte {
	return nil
}

// idleNormalizer replaces idle time fields with a placeholder so that
// minor timing differences between the two binary invocations do not
// cause spurious failures.
func idleNormalizer(b []byte) []byte {
	return idleFieldRe.ReplaceAll(b, []byte("${1}IDLE   ${3}"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gpinky")
	if err != nil {
		t.Skipf("reference binary gpinky not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: default invocation prints logged-in users in short format.
		{
			Name:      "default_no_args",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{idleNormalizer},
		},

		// R1.3: -s forces short format (same as default).
		{
			Name:      "short_format_flag",
			Args:      []string{"-s"},
			Normalize: []testutils.NormalizeFunc{idleNormalizer},
		},

		// R2.1: -l without operands requires at least one username.
		{
			Name:      "long_format_no_operands",
			Args:      []string{"-l"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R2.2: -f suppresses the header line in short format.
		{
			Name:      "suppress_header",
			Args:      []string{"-f"},
			Normalize: []testutils.NormalizeFunc{idleNormalizer},
		},

		// R2.3: -b suppresses home directory and shell in long format.
		{
			Name:      "long_suppress_home_no_operands",
			Args:      []string{"-lb"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R2.3: -h suppresses the project file in long format.
		{
			Name:      "long_suppress_project_no_operands",
			Args:      []string{"-lh"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R2.3: -p suppresses the plan file in long format.
		{
			Name:      "long_suppress_plan_no_operands",
			Args:      []string{"-lp"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R2.3: combined suppression flags in long format.
		{
			Name:      "long_suppress_all_no_operands",
			Args:      []string{"-lbhp"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// -w suppresses full name in short format.
		{
			Name:      "suppress_name_short",
			Args:      []string{"-w"},
			Normalize: []testutils.NormalizeFunc{idleNormalizer},
		},

		// R1.2: operand filtering — nonexistent user produces no output rows.
		{
			Name:      "operand_nonexistent_user",
			Args:      []string{"nonexistent_user_xyz_99"},
			Normalize: []testutils.NormalizeFunc{idleNormalizer},
		},

		// R1.2: -l with operand for nonexistent user shows ??? line.
		{
			Name: "long_nonexistent_user",
			Args: []string{"-l", "nonexistent_user_xyz_99"},
		},

		// R3.1/R3.3: --version exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.2: --help exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{discardOutput},
		},

		// R3.2: invalid flag exits 1 with error message.
		{
			Name:      "invalid_flag",
			Args:      []string{"--bogus"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},

		// R3.2: invalid short flag exits 1 with error message.
		{
			Name:      "invalid_short_flag",
			Args:      []string{"-Z"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
