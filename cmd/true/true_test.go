// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential and unit tests for cmd/true (prd013-true R4.1-R4.3).

package main_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
	"github.com/stretchr/testify/require"
)

// TestDiff runs differential tests against the gtrue reference binary.
// R4.1: compare exit codes. R4.2: cover no-args and arbitrary-args cases.
// R4.3: verify no output on stdout or stderr.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtrue")
	if err != nil {
		t.Skipf("reference binary gtrue not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			// R1.1: no arguments, exit 0, no output.
			Name:     "no_args",
			Args:     nil,
			ExitCode: 0,
		},
		{
			// R1.2: arbitrary arguments ignored, exit 0, no output.
			Name:     "arbitrary_args",
			Args:     []string{"foo", "bar", "--baz"},
			ExitCode: 0,
		},
		{
			// R1.2: single unknown flag ignored, exit 0, no output.
			Name:     "unknown_flag",
			Args:     []string{"--bogus"},
			ExitCode: 0,
		},
		{
			// R1.2: double dash then arguments, exit 0, no output.
			Name:     "double_dash_args",
			Args:     []string{"--", "anything"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHelp verifies --help prints usage to stdout and exits 0.
// R2.1: --help prints usage message. Not a differential test because
// GNU true includes distribution-specific text (URLs, copyright).
func TestHelp(t *testing.T) {
	t.Parallel()

	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--help")
	stdout, err := cmd.Output()
	require.NoError(t, err, "expected exit 0 for --help")

	out := string(stdout)
	require.True(t, strings.Contains(out, "Usage:"), "help output should contain Usage:")
	require.True(t, strings.Contains(out, "--help"), "help output should mention --help")
	require.True(t, strings.Contains(out, "--version"), "help output should mention --version")
}

// TestVersion verifies --version prints version info to stdout and exits 0.
// R2.2: --version prints version information.
func TestVersion(t *testing.T) {
	t.Parallel()

	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--version")
	stdout, err := cmd.Output()
	require.NoError(t, err, "expected exit 0 for --version")

	out := string(stdout)
	require.True(t, strings.Contains(out, "true"), "version output should contain 'true'")
}
