// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd063-timeout R1.1-R1.4 (core timeout behavior),
// R2.1-R2.4 (signal selection, kill-after, foreground, preserve-status),
// R3.1-R3.4 (exit codes: command status, timeout 124, signal 128+N, errors 125-127),
// and R4.1-R4.4 (differential testing, orphan cleanup verification).
package main

import (
	"bytes"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// clearOutput is a normalizer that discards all output, used for error
// tests where stderr differs between Go and reference binaries due to
// program name and quoting conventions but exit codes match.
var clearOutput testutils.NormalizeFunc = func(b []byte) []byte {
	return nil
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtimeout")
	if err != nil {
		t.Skipf("reference binary gtimeout not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R1.1: command completes before timeout, exit with command status 0.
		{
			Name:     "command_completes_exit_0",
			Args:     []string{"10", "true"},
			ExitCode: 0,
		},
		// R1.1: command completes before timeout, exit with command status 1.
		{
			Name:     "command_completes_exit_1",
			Args:     []string{"10", "false"},
			ExitCode: 1,
		},
		// R1.1: command exceeds timeout, killed with SIGTERM, exit 124.
		{
			Name:     "command_exceeds_timeout",
			Args:     []string{"0.01", "sleep", "10"},
			ExitCode: 124,
		},
		// R1.2: fractional duration with fast command.
		{
			Name:     "fractional_duration",
			Args:     []string{"0.5", "true"},
			ExitCode: 0,
		},
		// R1.4: duration 0 disables the timeout, command runs normally.
		{
			Name:     "duration_zero_no_limit",
			Args:     []string{"0", "true"},
			ExitCode: 0,
		},
		// R1.4: duration 0 with sleep 0 completes immediately. R4.3: fast.
		{
			Name:     "duration_zero_sleep_zero",
			Args:     []string{"0", "sleep", "0"},
			ExitCode: 0,
		},
		// R1.3: suffix 's' for seconds.
		{
			Name:     "suffix_seconds",
			Args:     []string{"10s", "true"},
			ExitCode: 0,
		},
		// R1.3: suffix 'm' for minutes.
		{
			Name:     "suffix_minutes",
			Args:     []string{"1m", "true"},
			ExitCode: 0,
		},
		// R1.3: suffix 'h' for hours.
		{
			Name:     "suffix_hours",
			Args:     []string{"1h", "true"},
			ExitCode: 0,
		},
		// R1.3: suffix 'd' for days.
		{
			Name:     "suffix_days",
			Args:     []string{"1d", "true"},
			ExitCode: 0,
		},
		// R1.2 + R1.3: fractional duration with suffix causes timeout.
		{
			Name:     "fractional_suffix_timeout",
			Args:     []string{"0.01s", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.1: -s with named signal KILL on timeout.
		{
			Name:     "signal_kill_named",
			Args:     []string{"-s", "KILL", "0.01", "sleep", "10"},
			ExitCode: 137,
		},
		// R2.1: -s with numeric signal 9 (KILL) on timeout.
		{
			Name:     "signal_kill_numeric",
			Args:     []string{"-s", "9", "0.01", "sleep", "10"},
			ExitCode: 137,
		},
		// R2.1: --signal= long form with equals.
		{
			Name:     "signal_long_form_eq",
			Args:     []string{"--signal=KILL", "0.01", "sleep", "10"},
			ExitCode: 137,
		},
		// R2.1: -s with command that completes before timeout.
		{
			Name:     "signal_no_timeout",
			Args:     []string{"-s", "KILL", "10", "true"},
			ExitCode: 0,
		},
		// R2.2: -k with process that dies from initial SIGTERM before kill-after fires.
		{
			Name:     "kill_after_not_triggered",
			Args:     []string{"-k", "100", "0.01", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.2: -k escalation with process that traps SIGTERM.
		{
			Name:     "kill_after_escalation",
			Args:     []string{"-k", "0.5", "0.01", "sh", "-c", "trap '' TERM; while :; do sleep 1; done"},
			ExitCode: 137,
		},
		// R2.3: --foreground with command that completes before timeout.
		{
			Name:     "foreground_completes",
			Args:     []string{"--foreground", "10", "true"},
			ExitCode: 0,
		},
		// R2.3: --foreground with command that exceeds timeout.
		{
			Name:     "foreground_timeout",
			Args:     []string{"--foreground", "0.01", "sleep", "10"},
			ExitCode: 124,
		},
		// R2.4: --preserve-status on timeout returns command's signal exit status.
		{
			Name:     "preserve_status_timeout",
			Args:     []string{"--preserve-status", "0.01", "sleep", "10"},
			ExitCode: 143,
		},
		// R2.4: --preserve-status without timeout returns command's exit code.
		{
			Name:     "preserve_status_no_timeout",
			Args:     []string{"--preserve-status", "10", "true"},
			ExitCode: 0,
		},
		// R2.4: --preserve-status with failed command.
		{
			Name:     "preserve_status_failed_cmd",
			Args:     []string{"--preserve-status", "10", "false"},
			ExitCode: 1,
		},
		// R3.1: command exits before timeout with non-zero status 2.
		{
			Name:     "r3_1_command_exit_status_2",
			Args:     []string{"10", "sh", "-c", "exit 2"},
			ExitCode: 2,
		},
		// R3.1: command exits before timeout with high exit code 42.
		{
			Name:     "r3_1_command_exit_status_42",
			Args:     []string{"10", "sh", "-c", "exit 42"},
			ExitCode: 42,
		},
		// R3.2: command killed due to timeout exits 124.
		{
			Name:     "r3_2_timeout_exit_124",
			Args:     []string{"0.01", "sleep", "60"},
			ExitCode: 124,
		},
		// R3.3: command killed by signal not sent by timeout. Uses
		// --foreground to avoid process group complications. The child
		// kills itself with SIGHUP; timeout re-raises the signal.
		{
			Name:      "r3_3_signal_not_from_timeout",
			Args:      []string{"--foreground", "10", "sh", "-c", "kill -HUP $$"},
			ExitCode:  129,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R3.4: invalid duration → exit 125. Stderr normalizer ignores
		// program name and quoting differences.
		{
			Name:      "r3_4_invalid_duration",
			Args:      []string{"abc", "true"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R3.4: command not found → exit 127. Stderr normalizer ignores
		// error message format differences.
		{
			Name:      "r3_4_command_not_found",
			Args:      []string{"10", "nonexistent_command_xyz_12345"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R3.4: missing operand (only duration, no command) → exit 125.
		{
			Name:      "r3_4_missing_operand",
			Args:      []string{"10"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R4.2: no args at all → exit 125. Covers the "no args" error case.
		{
			Name:      "r4_2_no_args",
			Args:      []string{},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R3.2 + R2.2: kill-after with timeout, exit 137 after SIGKILL escalation.
		{
			Name:     "r3_2_kill_after_exit_137",
			Args:     []string{"-k", "0.5", "0.01", "sh", "-c", "trap '' TERM; while :; do sleep 1; done"},
			ExitCode: 137,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestOrphanCleanup verifies that the timed-out child process is actually
// killed and does not remain as an orphan. R4.4.
func TestOrphanCleanup(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	// Run timeout with a short duration. The child shell writes its PID
	// to stdout, then exec's sleep to keep the same PID alive.
	cmd := exec.Command(goBin, "0.1", "sh", "-c", "echo $$; exec sleep 60")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		// Expected: timeout exits non-zero (124).
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("unexpected error type: %v", err)
		}
		if exitErr.ExitCode() != 124 {
			t.Fatalf("expected exit code 124, got %d", exitErr.ExitCode())
		}
	}
	pidStr := strings.TrimSpace(stdout.String())
	if pidStr == "" {
		t.Fatal("child did not write PID to stdout")
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatalf("invalid PID %q: %v", pidStr, err)
	}
	// Brief pause to allow the OS to fully reap the process.
	time.Sleep(100 * time.Millisecond)
	// kill -0 checks if the process exists without sending a signal.
	// ESRCH means the process is gone (expected). Any other result means
	// the child survived the timeout and is an orphan.
	err = syscall.Kill(pid, 0)
	if err == nil {
		t.Fatalf("R4.4: process %d still running after timeout — orphan detected", pid)
	}
}
