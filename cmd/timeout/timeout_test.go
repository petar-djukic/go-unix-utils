// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/timeout against GNU gtimeout.
// Covers prd063-timeout R4.1-R4.4 (differential testing).
package main

import (
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes error messages between GNU gtimeout and Go timeout.
// GNU and Go format error details differently; this normalizer strips
// implementation-specific text so exit code comparison drives the test.
func stderrNormalizer() testutils.NormalizeFunc {
	binPath := regexp.MustCompile(`/[^\s:]+/g?timeout|gtimeout`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	cmdFail := regexp.MustCompile(
		`(?m)^timeout: failed to run command.*\n?`)
	sigErr := regexp.MustCompile(
		`(?m)^timeout:.*invalid signal.*\n?`)
	missingOp := regexp.MustCompile(
		`(?m)^timeout: missing operand.*\n?`)
	return func(b []byte) []byte {
		b = binPath.ReplaceAll(b, []byte("timeout"))
		b = tryHelp.ReplaceAll(b, nil)
		b = cmdFail.ReplaceAll(b, nil)
		b = sigErr.ReplaceAll(b, nil)
		b = missingOp.ReplaceAll(b, nil)
		return b
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtimeout")
	if err != nil {
		t.Skipf("reference binary gtimeout not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	tests := buildBasicTests()
	tests = append(tests, buildSignalTests(errNorm)...)
	tests = append(tests, buildKillAndStatusTests()...)
	tests = append(tests, buildErrorTests(errNorm)...)
	tests = append(tests, buildEdgeTests()...)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildBasicTests returns test cases for R4.1: basic timeout behavior.
func buildBasicTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		// R4.1/R3.1: command completes before timeout, exit with command status.
		{
			Name: "command_completes_exit_0",
			Args: []string{"10", "true"},
		},
		// R4.1/R3.1: command exits non-zero before timeout.
		{
			Name: "command_completes_exit_1",
			Args: []string{"10", "false"},
		},
		// R4.1/R3.2: command exceeds timeout, exit 124.
		{
			Name: "command_exceeds_timeout",
			Args: []string{"0.1", "sleep", "10"},
		},
		// R4.1/R1.2: fractional duration, command completes before timeout.
		{
			Name: "fractional_duration_completes",
			Args: []string{"0.5", "true"},
		},
	}
}

// buildSignalTests returns test cases for R4.2/R2.1: signal selection.
// Note: SIGKILL tests are excluded because GNU timeout kills its own process
// group with SIGKILL (exits 137) while our implementation only kills the
// child's group (exits 124). This difference is documented in non_goals.
func buildSignalTests(errNorm testutils.NormalizeFunc) []testutils.DiffTest {
	return []testutils.DiffTest{
		// R4.2/R2.1: -s with named signal INT.
		{
			Name: "signal_name_INT",
			Args: []string{"-s", "INT", "0.1", "sleep", "10"},
		},
		// R4.2/R2.1: -s with numeric signal 1 (SIGHUP).
		{
			Name: "signal_numeric_1",
			Args: []string{"-s", "1", "0.1", "sleep", "10"},
		},
		// R4.2/R2.1: -s with signal name HUP.
		{
			Name: "signal_name_HUP",
			Args: []string{"-s", "HUP", "0.1", "sleep", "10"},
		},
		// R4.2/R2.1: --signal= long form.
		{
			Name: "signal_long_form",
			Args: []string{"--signal=TERM", "0.1", "sleep", "10"},
		},
		// R4.2/R2.1: invalid signal name triggers error.
		{
			Name:      "signal_invalid_name",
			Args:      []string{"-s", "NOTASIGNAL", "1", "true"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
}

// buildKillAndStatusTests returns test cases for R4.2: -k and --preserve-status.
func buildKillAndStatusTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		// R4.2/R2.2: -k kill-after escalation.
		{
			Name: "kill_after_short",
			Args: []string{"-k", "0.1", "0.1", "sleep", "10"},
		},
		// R4.2/R2.2: --kill-after= long form.
		{
			Name: "kill_after_long_form",
			Args: []string{"--kill-after=0.1", "0.1", "sleep", "10"},
		},
		// R4.2/R2.4: --preserve-status on timeout returns signal exit code.
		{
			Name: "preserve_status_timeout",
			Args: []string{"--preserve-status", "0.1", "sleep", "10"},
		},
		// R4.2/R2.4: --preserve-status when command succeeds.
		{
			Name: "preserve_status_success",
			Args: []string{"--preserve-status", "10", "true"},
		},
		// R4.2/R2.4: --preserve-status when command fails.
		{
			Name: "preserve_status_failure",
			Args: []string{"--preserve-status", "10", "false"},
		},
	}
}

// buildErrorTests returns test cases for R4.3: error conditions.
func buildErrorTests(errNorm testutils.NormalizeFunc) []testutils.DiffTest {
	return []testutils.DiffTest{
		// R4.3/R3.4: no arguments at all.
		{
			Name:      "error_no_args",
			Args:      []string{},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3/R3.4: duration only, missing command.
		{
			Name:      "error_missing_command",
			Args:      []string{"1"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3/R3.4: invalid duration string.
		{
			Name:      "error_invalid_duration",
			Args:      []string{"abc", "true"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R4.3/R3.4: command not found (exit 127).
		{
			Name:      "error_command_not_found",
			Args:      []string{"1", "nonexistent_command_xyz_42"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}
}

// buildEdgeTests returns test cases for R4.4: edge cases.
func buildEdgeTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		// R4.4/R1.4: duration 0 disables timeout, command exits normally.
		{
			Name: "zero_duration_no_timeout",
			Args: []string{"0", "true"},
		},
		// R4.4/R1.4: duration 0 with sleep 0.
		{
			Name: "zero_duration_sleep_zero",
			Args: []string{"0", "sleep", "0"},
		},
		// R4.4/R2.3: --foreground mode, command completes.
		{
			Name: "foreground_completes",
			Args: []string{"--foreground", "10", "true"},
		},
		// R4.4/R2.3: --foreground mode, command times out.
		{
			Name: "foreground_timeout",
			Args: []string{"--foreground", "0.1", "sleep", "10"},
		},
		// R4.4/R1.2: very short fractional duration.
		{
			Name: "very_short_fractional",
			Args: []string{"0.01", "sleep", "10"},
		},
		// R4.4/R1.3: duration with suffix s (seconds).
		{
			Name: "suffix_seconds",
			Args: []string{"10s", "true"},
		},
		// R4.4: command with arguments passes args through.
		{
			Name: "command_with_args",
			Args: []string{"10", "sleep", "0"},
		},
		// R4.4: -- separator before positional arguments.
		{
			Name: "double_dash_separator",
			Args: []string{"--", "10", "true"},
		},
	}
}
