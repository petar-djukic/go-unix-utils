// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNamePattern matches a binary path prefix in error messages.
var binaryNamePattern = regexp.MustCompile(`(?m)^[^\s:]*df: `)

// tryHelpPattern matches the GNU "Try '...' for more information." line.
var tryHelpPattern = regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)

// digitPattern matches sequences of digits for normalizing volatile numeric output.
var digitPattern = regexp.MustCompile(`\d+`)

// normalizeBinaryName replaces any binary path prefix ending in df: with df:
// so stderr from the reference binary matches our Go binary's error format.
func normalizeBinaryName(b []byte) []byte {
	return binaryNamePattern.ReplaceAll(b, []byte("df: "))
}

// stripTryHelp removes the GNU help hint line from stderr output.
func stripTryHelp(b []byte) []byte {
	return tryHelpPattern.ReplaceAll(b, nil)
}

// normalizeDigits replaces digit sequences with a placeholder so volatile
// filesystem statistics (block counts, inode counts) do not cause false failures.
func normalizeDigits(b []byte) []byte {
	return digitPattern.ReplaceAll(b, []byte("N"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdf")
	if err != nil {
		t.Skipf("reference binary gdf not in PATH: %v", err)
	}

	errorNorm := []testutils.NormalizeFunc{normalizeBinaryName, stripTryHelp}
	volatileNorm := []testutils.NormalizeFunc{normalizeDigits}
	allNorm := []testutils.NormalizeFunc{normalizeBinaryName, stripTryHelp, normalizeDigits}

	tests := []testutils.DiffTest{
		// R4.3: basic invocation with / validates SIGPIPE handler and exit 0.
		{
			Name:      "root_filesystem",
			Args:      []string{"/"},
			ExitCode:  0,
			Env:       []string{"LC_ALL=C"},
			Normalize: volatileNorm,
		},
		// R4.2: non-existent file produces diagnostic on stderr and exits 1.
		{
			Name:      "nonexistent_file_exits_1",
			Args:      []string{"/nonexistent_df_test_path_xyz"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: errorNorm,
		},
		// R4.2: invalid short option produces diagnostic and exits 1.
		{
			Name:      "invalid_short_option",
			Args:      []string{"-Q"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: errorNorm,
		},
		// R4.2: invalid long option produces diagnostic and exits 1.
		{
			Name:      "invalid_long_option",
			Args:      []string{"--nonexistent-flag"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: errorNorm,
		},
		// R4.2: mix of valid and invalid file args still exits 1.
		{
			Name:      "mixed_valid_invalid_files",
			Args:      []string{"/", "/nonexistent_df_test_path_xyz"},
			ExitCode:  1,
			Env:       []string{"LC_ALL=C"},
			Normalize: allNorm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
