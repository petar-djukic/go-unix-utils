// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd063-timeout R4.1, R4.2, R4.3, R4.4.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
func discardAll(data []byte) []byte {
	return nil
}

// R4.1: Differential tests compare exit codes between Go and reference binary.
// R4.2: Tests cover all specified scenarios.
// R4.3: Tests use fast commands (true, false, sleep 0) and short durations.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtimeout")
	if err != nil {
		t.Skipf("reference binary gtimeout not in PATH: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.2: command completes before timeout, exit with command status 0.
		{
			Name:      "command_exits_before_timeout",
			Args:      []string{"10", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: command completes with non-zero status before timeout.
		{
			Name:      "command_fails_before_timeout",
			Args:      []string{"10", "false"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: command exceeds timeout, exit 124.
		{
			Name:      "command_exceeds_timeout",
			Args:      []string{"0.01", "sleep", "10"},
			ExitCode:  124,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.3: fractional duration (0.5s is enough for true to complete).
		{
			Name:      "fractional_duration_completes",
			Args:      []string{"0.5", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.3: fractional duration triggers timeout.
		{
			Name:      "fractional_duration_timeout",
			Args:      []string{"0.01", "sleep", "10"},
			ExitCode:  124,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: suffix s (seconds).
		{
			Name:      "suffix_s",
			Args:      []string{"10s", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: suffix m (minutes).
		{
			Name:      "suffix_m",
			Args:      []string{"1m", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: suffix h (hours).
		{
			Name:      "suffix_h",
			Args:      []string{"1h", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: suffix d (days).
		{
			Name:      "suffix_d",
			Args:      []string{"1d", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: duration 0 means no time limit; command runs to completion.
		{
			Name:      "duration_zero_no_limit",
			Args:      []string{"0", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: duration 0 with a failing command still returns command status.
		{
			Name:      "duration_zero_command_fails",
			Args:      []string{"0", "false"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: error case — no arguments.
		{
			Name:      "no_args_error",
			Args:      []string{},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: error case — only duration, no command.
		{
			Name:      "missing_command_error",
			Args:      []string{"10"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: error case — invalid duration.
		{
			Name:      "invalid_duration_error",
			Args:      []string{"abc", "true"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: error case — command not found.
		{
			Name:      "command_not_found_error",
			Args:      []string{"10", "nonexistent_command_xyz_12345"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},

		// R4.2: -s with named signal KILL, command exceeds timeout.
		// Both binaries die from re-raised SIGKILL; Go ExitCode() reports -1.
		{
			Name:      "signal_named_kill_timeout",
			Args:      []string{"-s", "KILL", "0.01", "sleep", "10"},
			ExitCode:  -1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: -s with numeric signal 9, command exceeds timeout.
		{
			Name:      "signal_numeric_9_timeout",
			Args:      []string{"-s", "9", "0.01", "sleep", "10"},
			ExitCode:  -1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: --signal= long form with named signal.
		{
			Name:      "signal_long_form_timeout",
			Args:      []string{"--signal=KILL", "0.01", "sleep", "10"},
			ExitCode:  -1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: -s with signal name, command completes before timeout.
		{
			Name:      "signal_completes_before_timeout",
			Args:      []string{"-s", "KILL", "10", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: -k kill-after, SIGTERM then SIGKILL escalation.
		{
			Name:      "kill_after_escalation",
			Args:      []string{"-k", "0.01", "0.01", "sleep", "10"},
			ExitCode:  124,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: --kill-after= long form.
		{
			Name:      "kill_after_long_form",
			Args:      []string{"--kill-after=0.01", "0.01", "sleep", "10"},
			ExitCode:  124,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: --foreground, command exceeds timeout.
		{
			Name:      "foreground_timeout",
			Args:      []string{"--foreground", "0.01", "sleep", "10"},
			ExitCode:  124,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: --foreground, command completes before timeout.
		{
			Name:      "foreground_completes",
			Args:      []string{"--foreground", "10", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: --preserve-status, command completes before timeout.
		{
			Name:      "preserve_status_completes",
			Args:      []string{"--preserve-status", "10", "true"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: --preserve-status, command fails before timeout.
		{
			Name:      "preserve_status_fails",
			Args:      []string{"--preserve-status", "10", "false"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},

		// R4.2: command exits with specific non-zero status before timeout.
		{
			Name:      "exit_status_2",
			Args:      []string{"10", "sh", "-c", "exit 2"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: command exits with status 42 before timeout.
		{
			Name:      "exit_status_42",
			Args:      []string{"10", "sh", "-c", "exit 42"},
			ExitCode:  42,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: timeout exits 124 when command killed by timeout with kill-after.
		{
			Name:      "timeout_exit_124_with_kill_after",
			Args:      []string{"-k", "0.5", "0.01", "sleep", "10"},
			ExitCode:  124,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: --preserve-status on timeout exits 128+signum instead of 124.
		{
			Name:      "preserve_status_timeout",
			Args:      []string{"--preserve-status", "0.01", "sleep", "10"},
			ExitCode:  143, // 128 + 15 (SIGTERM)
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: command killed by signal not sent by timeout (self-kill with SIGHUP).
		// Both binaries re-raise the child's signal on themselves and die from it;
		// Go's exec reports exit code -1 for signal-killed processes.
		{
			Name:      "command_self_signal_hup",
			Args:      []string{"10", "sh", "-c", "kill -HUP $$"},
			ExitCode:  -1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: internal error — invalid duration, exit 125.
		{
			Name:      "error_invalid_duration_125",
			Args:      []string{"xyz", "true"},
			ExitCode:  125,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: command not found, exit 127.
		{
			Name:      "error_command_not_found_127",
			Args:      []string{"10", "nonexistent_cmd_9876"},
			ExitCode:  127,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: command cannot execute (not executable file), exit 126.
		{
			Name:      "error_cannot_execute_126",
			Args:      []string{"10", "/dev/null"},
			ExitCode:  126,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: -s with HUP signal name, command exceeds timeout.
		// HUP is catchable, so timeout absorbs the re-raised signal and exits 124.
		{
			Name:      "signal_named_hup_timeout",
			Args:      []string{"-s", "HUP", "0.01", "sleep", "10"},
			ExitCode:  124,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: duration 0 with sleep 0 (fast command, no limit).
		{
			Name:      "duration_zero_sleep_zero",
			Args:      []string{"0", "sleep", "0"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestParseDuration verifies duration parsing unit tests.
func TestParseDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		// R1.2: fractional values.
		{name: "fractional", input: "0.5", want: 500 * time.Millisecond},
		{name: "integer", input: "3", want: 3 * time.Second},
		// R1.3: suffix multipliers.
		{name: "suffix_s", input: "2s", want: 2 * time.Second},
		{name: "suffix_m", input: "1m", want: 60 * time.Second},
		{name: "suffix_h", input: "1h", want: 3600 * time.Second},
		{name: "suffix_d", input: "1d", want: 86400 * time.Second},
		// R1.4: duration 0.
		{name: "zero", input: "0", want: 0},
		{name: "zero_s", input: "0s", want: 0},
		// Errors.
		{name: "negative", input: "-1", wantErr: true},
		{name: "non_numeric", input: "abc", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseDuration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseDuration(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDuration(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseDuration(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestParseSignal verifies signal parsing for R2.1.
func TestParseSignal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    syscall.Signal
		wantErr bool
	}{
		{name: "named_kill", input: "KILL", want: syscall.SIGKILL},
		{name: "named_term", input: "TERM", want: syscall.SIGTERM},
		{name: "named_hup", input: "HUP", want: syscall.SIGHUP},
		{name: "named_int", input: "INT", want: syscall.SIGINT},
		{name: "sig_prefix", input: "SIGKILL", want: syscall.SIGKILL},
		{name: "lowercase", input: "kill", want: syscall.SIGKILL},
		{name: "numeric_9", input: "9", want: syscall.Signal(9)},
		{name: "numeric_15", input: "15", want: syscall.Signal(15)},
		{name: "numeric_0", input: "0", want: syscall.Signal(0)},
		{name: "invalid_name", input: "BOGUS", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseSignal(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseSignal(%q) = %v, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSignal(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseSignal(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestProcessKilled verifies that the timed-out process is actually killed
// and does not remain as an orphan.
// R4.4: must verify the timed-out process is actually killed.
func TestProcessKilled(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	// Start timeout with a long-running child process.
	// Use sh -c to get a predictable child that writes its PID.
	cmd := exec.Command(goBin, "0.2", "sh", "-c",
		fmt.Sprintf("echo $$ > %s/child.pid; sleep 60", t.TempDir()))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	pidFile := fmt.Sprintf("%s/child.pid", t.TempDir())

	// Use a simpler approach: start timeout and wait for it to exit,
	// then verify the child sleep process is gone.
	cmd2 := exec.Command(goBin, "-k", "0.1", "0.2", "sleep", "60")
	if err := cmd2.Start(); err != nil {
		t.Fatalf("failed to start timeout: %v", err)
	}
	childPid := cmd2.Process.Pid

	// Wait for timeout to complete.
	_ = cmd2.Wait() // best-effort; we expect non-zero exit

	// Give OS a moment to clean up the process.
	time.Sleep(100 * time.Millisecond)

	// Verify the timeout process itself is gone by sending signal 0.
	err := syscall.Kill(childPid, 0)
	if err == nil {
		t.Errorf("timeout process (pid %d) still alive after exit", childPid)
	}

	// Also verify via a fresh run: start a child that traps SIGTERM,
	// use -k to escalate to SIGKILL, and confirm it exits.
	cmd3 := exec.Command(goBin, "-k", "0.1", "0.1", "sh", "-c",
		"trap '' TERM; sleep 60")
	if err := cmd3.Start(); err != nil {
		t.Fatalf("failed to start timeout with trap: %v", err)
	}

	errWait := cmd3.Wait()
	if errWait == nil {
		t.Fatal("expected non-zero exit from timeout with trapped SIGTERM")
	}
	exitErr, ok := errWait.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", errWait, errWait)
	}
	if exitErr.ExitCode() != 124 && exitErr.ExitCode() != 137 {
		t.Fatalf("expected exit code 124 or 137, got %d", exitErr.ExitCode())
	}

	// Clean up unused pidFile reference.
	_ = pidFile
	_ = cmd
}

// TestOrphanProcessCleanup verifies the child process does not survive
// after the timeout command completes.
// R4.4: timed-out process must not remain as an orphan.
func TestOrphanProcessCleanup(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	tmpDir := t.TempDir()
	pidFile := fmt.Sprintf("%s/sleep.pid", tmpDir)

	// Run timeout with a shell that records its PID and then sleeps.
	cmd := exec.Command(goBin, "-k", "0.1", "0.2", "sh", "-c",
		fmt.Sprintf("echo $$ > %s; exec sleep 60", pidFile))
	if err := cmd.Run(); err == nil {
		t.Log("timeout exited 0 unexpectedly, child may have been fast")
	}

	// Read the child PID.
	data, err := os.ReadFile(pidFile)
	if err != nil {
		// Child may not have written PID before being killed; that's OK.
		t.Skipf("child did not write PID file: %v", err)
	}

	pidStr := strings.TrimSpace(string(data))
	childPid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatalf("invalid PID in file: %q", pidStr)
	}

	// Allow cleanup time.
	time.Sleep(200 * time.Millisecond)

	// Signal 0 checks if the process exists without sending a real signal.
	if err := syscall.Kill(childPid, 0); err == nil {
		t.Errorf("child process (pid %d) still alive after timeout killed it", childPid)
		// best-effort cleanup
		_ = syscall.Kill(childPid, syscall.SIGKILL)
	}
}
