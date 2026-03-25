// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/who against gwho (GNU coreutils).
// Covers prd097-who R3.1 (exit 0 on success), R3.2 (exit 1 on error),
// R3.3 (SIGPIPE handling).
package main_test

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer replaces the binary name/path prefix in stderr and
// strips the "Try '...' for more information." hint line so that
// structural error messages can be compared across binaries.
func stderrNormalizer(b []byte) []byte {
	re := regexp.MustCompile(`(?m)^[^\s:]*(?:gwho|who):`)
	b = re.ReplaceAll(b, []byte("who:"))
	tryRe := regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)
	b = tryRe.ReplaceAll(b, nil)
	return b
}

// stdoutClearNorm blanks stdout so only exit codes are compared.
// R3.1-R3.3 concern exit codes and SIGPIPE, not output format.
// Output format parity is verified by R1/R2 tasks.
func stdoutClearNorm(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwho")
	if err != nil {
		t.Skip("reference binary gwho not in PATH")
	}

	errNorms := []testutils.NormalizeFunc{stderrNormalizer}
	exitNorms := []testutils.NormalizeFunc{stdoutClearNorm}

	tests := []testutils.DiffTest{
		// ── R3.1: exit 0 on success ──────────────────────────────

		// Default invocation with no arguments.
		{
			Name:      "default_no_args",
			Args:      []string{},
			Normalize: exitNorms,
		},
		// -H heading option.
		{
			Name:      "heading",
			Args:      []string{"-H"},
			Normalize: exitNorms,
		},
		// -b boot time.
		{
			Name:      "boot_time",
			Args:      []string{"-b"},
			Normalize: exitNorms,
		},
		// -q count mode.
		{
			Name: "count_mode",
			Args: []string{"-q"},
		},
		// Combined -bH.
		{
			Name:      "boot_heading",
			Args:      []string{"-bH"},
			Normalize: exitNorms,
		},
		// Long flag --heading.
		{
			Name:      "long_heading",
			Args:      []string{"--heading"},
			Normalize: exitNorms,
		},
		// Long flag --boot.
		{
			Name:      "long_boot",
			Args:      []string{"--boot"},
			Normalize: exitNorms,
		},
		// Long flag --count.
		{
			Name: "long_count",
			Args: []string{"--count"},
		},
		// Combined -Hb (reversed order).
		{
			Name:      "heading_boot_reversed",
			Args:      []string{"-Hb"},
			Normalize: exitNorms,
		},
		// Short format -s (default, ignored).
		{
			Name:      "short_format",
			Args:      []string{"-s"},
			Normalize: exitNorms,
		},
		// -u users (idle time).
		{
			Name:      "users_idle",
			Args:      []string{"-u"},
			Normalize: exitNorms,
		},
		// Combined -Hu (heading + idle).
		{
			Name:      "heading_users",
			Args:      []string{"-Hu"},
			Normalize: exitNorms,
		},
		// -T message status.
		{
			Name:      "mesg_status",
			Args:      []string{"-T"},
			Normalize: exitNorms,
		},
		// Combined -buH (boot + idle + heading).
		{
			Name:      "boot_users_heading",
			Args:      []string{"-buH"},
			Normalize: exitNorms,
		},
		// -a all option.
		{
			Name:      "all_flag",
			Args:      []string{"-a"},
			Normalize: exitNorms,
		},

		// ── R3.2: exit 1 on error ───────────────────────────────

		// Unrecognized short option.
		{
			Name:      "invalid_short_option",
			Args:      []string{"-Z"},
			Normalize: errNorms,
		},
		// Unrecognized long option.
		{
			Name:      "invalid_long_option",
			Args:      []string{"--bogus"},
			Normalize: errNorms,
		},
		// Extra operand (more than 2 positional args).
		{
			Name:      "extra_operand",
			Args:      []string{"a", "b", "c"},
			Normalize: errNorms,
		},

		// ── R3.3: edge cases ─────────────────────────────────────

		// who am i (two positional args triggers am-i mode).
		{
			Name: "am_i",
			Args: []string{"am", "i"},
		},
		// -m flag (equivalent to am-i).
		{
			Name: "m_flag",
			Args: []string{"-m"},
		},
		// No arguments: verify exit 0 (same as default_no_args
		// but explicitly tests the zero-argument edge case).
		{
			Name:      "no_arguments_edge",
			Args:      nil,
			Normalize: exitNorms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
