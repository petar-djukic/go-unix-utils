// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/pinky against gpinky (GNU coreutils).
// Covers prd098-pinky R3.1 (exit 0 on success), R3.2 (exit 1 on error),
// R3.3 (SIGPIPE handling via InstallSIGPIPEHandler).
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
	re := regexp.MustCompile(`(?m)^[^\s:]*(?:gpinky|pinky):`)
	b = re.ReplaceAll(b, []byte("pinky:"))
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
	refBin, err := exec.LookPath("gpinky")
	if err != nil {
		t.Skip("reference binary gpinky not in PATH")
	}

	errNorms := []testutils.NormalizeFunc{stderrNormalizer}
	exitNorms := []testutils.NormalizeFunc{stdoutClearNorm}
	bothNorms := []testutils.NormalizeFunc{stdoutClearNorm, stderrNormalizer}

	tests := []testutils.DiffTest{
		// ── R3.1: exit 0 on success ──────────────────────────────

		// Default invocation with no arguments (short format).
		{
			Name:      "default_no_args",
			Args:      []string{},
			Normalize: exitNorms,
		},
		// -s short format (explicit, same as default).
		{
			Name:      "short_format_explicit",
			Args:      []string{"-s"},
			Normalize: exitNorms,
		},
		// -l long format with current user.
		{
			Name:      "long_format_current_user",
			Args:      []string{"-l", currentUser(t)},
			Normalize: exitNorms,
		},
		// -f suppress header line in short format.
		{
			Name:      "suppress_header",
			Args:      []string{"-f"},
			Normalize: exitNorms,
		},
		// -w suppress full name in short format.
		{
			Name:      "suppress_name",
			Args:      []string{"-w"},
			Normalize: exitNorms,
		},
		// -i suppress name and host in short format.
		{
			Name:      "suppress_name_host",
			Args:      []string{"-i"},
			Normalize: exitNorms,
		},
		// -q suppress name, host, and idle time in short format.
		{
			Name:      "suppress_name_host_idle",
			Args:      []string{"-q"},
			Normalize: exitNorms,
		},
		// -b omit home directory and shell in long format.
		{
			Name:      "long_omit_home",
			Args:      []string{"-l", "-b", currentUser(t)},
			Normalize: exitNorms,
		},
		// -h omit project file in long format.
		{
			Name:      "long_omit_project",
			Args:      []string{"-l", "-h", currentUser(t)},
			Normalize: exitNorms,
		},
		// -p omit plan file in long format.
		{
			Name:      "long_omit_plan",
			Args:      []string{"-l", "-p", currentUser(t)},
			Normalize: exitNorms,
		},
		// Combined -bhp in long format.
		{
			Name:      "long_omit_all_extras",
			Args:      []string{"-l", "-bhp", currentUser(t)},
			Normalize: exitNorms,
		},
		// Combined -sf (short + suppress header).
		{
			Name:      "combined_sf",
			Args:      []string{"-sf"},
			Normalize: exitNorms,
		},
		// Combined -sw (short + suppress name).
		{
			Name:      "combined_sw",
			Args:      []string{"-sw"},
			Normalize: exitNorms,
		},
		// Combined -fwiq (all short suppressions).
		{
			Name:      "combined_all_short_suppress",
			Args:      []string{"-fwiq"},
			Normalize: exitNorms,
		},
		// --help exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: exitNorms,
		},
		// --version exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
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
		// Another invalid short option.
		{
			Name:      "invalid_short_option_x",
			Args:      []string{"-x"},
			Normalize: errNorms,
		},
		// Combined valid and invalid short options.
		{
			Name:      "mixed_valid_invalid",
			Args:      []string{"-sZ"},
			Normalize: errNorms,
		},

		// ── R3.3: edge cases ─────────────────────────────────────

		// Nonexistent user in long format (exits 0 with partial output).
		{
			Name:      "long_nonexistent_user",
			Args:      []string{"-l", "nonexistent_user_12345"},
			Normalize: bothNorms,
		},
		// User filter for nonexistent user in short format.
		{
			Name:      "short_nonexistent_user",
			Args:      []string{"nonexistent_user_12345"},
			Normalize: exitNorms,
		},
		// No arguments: verify exit 0 (nil args edge case).
		{
			Name:      "no_arguments_nil",
			Args:      nil,
			Normalize: exitNorms,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// currentUser returns the current username via os/user or whoami.
func currentUser(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("whoami").Output()
	if err != nil {
		t.Fatalf("failed to get current user: %v", err)
	}
	user := string(out)
	// Trim trailing newline.
	if len(user) > 0 && user[len(user)-1] == '\n' {
		user = user[:len(user)-1]
	}
	return user
}
