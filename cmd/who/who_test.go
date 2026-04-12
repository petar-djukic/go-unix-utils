// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main provides differential tests for cmd/who.
// Tests cover srd097-who R1 (core output), R2 (display options), R3 (error/version/help).
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

// idleFieldRe matches the idle time field in who -u output.
// Captures: (1) time + separator, (2) idle value (5 chars), (3) PID field.
// Idle time may differ between the two binary invocations because
// terminal device mod times change when a process writes to the terminal.
var idleFieldRe = regexp.MustCompile(
	`((?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}\s)` +
		`((?:\d{2}:\d{2}| old |  \.  |  \?  ))` +
		`(\s+\d+)`,
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
// cause spurious failures in who -u tests.
func idleNormalizer(b []byte) []byte {
	return idleFieldRe.ReplaceAll(b, []byte("${1}IDLE ${3}"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwho")
	if err != nil {
		t.Skipf("reference binary gwho not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R1.1: default invocation prints logged-in users with terminal and time.
		{
			Name: "default_no_args",
			Args: []string{},
		},

		// R2.1: -H prints a header line above the output.
		{
			Name: "heading_short",
			Args: []string{"-H"},
		},

		// R2.1: --heading long form.
		{
			Name: "heading_long",
			Args: []string{"--heading"},
		},

		// R2.4: -q prints login names and user count.
		{
			Name: "count_short",
			Args: []string{"-q"},
		},

		// R2.4: --count long form.
		{
			Name: "count_long",
			Args: []string{"--count"},
		},

		// R2.3: -b prints boot time.
		{
			Name: "boot_short",
			Args: []string{"-b"},
		},

		// R2.3: --boot long form.
		{
			Name: "boot_long",
			Args: []string{"--boot"},
		},

		// R2.2: -u shows idle time for each user.
		{
			Name:      "users_short",
			Args:      []string{"-u"},
			Normalize: []testutils.NormalizeFunc{idleNormalizer},
		},

		// R2.2: --users long form.
		{
			Name:      "users_long",
			Args:      []string{"--users"},
			Normalize: []testutils.NormalizeFunc{idleNormalizer},
		},

		// R1.3: "am i" two-argument form prints current user's entry.
		{
			Name: "am_i",
			Args: []string{"am", "i"},
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
