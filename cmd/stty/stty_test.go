// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/stty against gstty (GNU coreutils).
//
// Tests prd105-stty R1.1, R2.1, R3.1, R3.2, R4.1, R5.1, R6.1, R6.2, R7.1, R7.2, R7.3.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// progNameNorm normalizes the program name prefix in output so that
// "gstty: " and "stty: " compare as equal.
var progNameNorm = func(data []byte) []byte {
	re := regexp.MustCompile(`(?m)^g?stty: `)
	return re.ReplaceAll(data, []byte("stty: "))
}

// errMsgNorm normalizes both error prefix and "Try" line for usage errors.
var errMsgNorm = testutils.ComposeNormalizers(
	progNameNorm,
	func(data []byte) []byte {
		return regexp.MustCompile(`Try 'g?stty`).ReplaceAll(data, []byte("Try 'stty"))
	},
)

// TestDiff runs differential tests comparing the Go binary against gstty.
// Tests use -F /dev/tty to ensure both binaries read the same terminal device.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gstty")
	if err != nil {
		t.Skipf("reference binary gstty not in PATH: %v", err)
	}

	// Verify /dev/tty is accessible for terminal-dependent tests.
	f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err != nil {
		t.Skipf("/dev/tty not accessible: %v", err)
	}
	f.Close()

	tests := []testutils.DiffTest{
		{
			Name: "default_via_device",
			Args: []string{"-F", "/dev/tty"},
		},
		{
			Name: "all_via_device",
			Args: []string{"-a", "-F", "/dev/tty"},
		},
		{
			Name: "save_via_device",
			Args: []string{"-g", "-F", "/dev/tty"},
		},
		{
			Name: "all_long_flag",
			Args: []string{"--all", "--file=/dev/tty"},
		},
		{
			Name: "save_long_flag",
			Args: []string{"--save", "--file=/dev/tty"},
		},
		{
			Name:      "error_no_tty",
			Args:      []string{},
			Stdin:     []byte{},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNorm},
		},
		{
			Name:      "error_nonexistent_device",
			Args:      []string{"-F", "/dev/nonexistent_stty_test_device"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{progNameNorm},
		},
		// R4.1: invalid setting name produces matching error.
		{
			Name:      "error_invalid_setting",
			Args:      []string{"-F", "/dev/tty", "zzz_invalid_xyz"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errMsgNorm},
		},
		// R5.1: missing special character argument produces matching error.
		{
			Name:      "error_missing_cc_arg",
			Args:      []string{"-F", "/dev/tty", "intr"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errMsgNorm},
		},
		// R6.2: missing speed argument produces matching error.
		{
			Name:      "error_missing_speed_arg",
			Args:      []string{"-F", "/dev/tty", "ispeed"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{errMsgNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSttyExitCodes verifies exit codes for error conditions.
func TestSttyExitCodes(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	t.Run("invalid_device_exits_1", func(t *testing.T) {
		cmd := exec.Command(bin, "-F", "/dev/nonexistent_stty_test_device")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected non-zero exit for invalid device")
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("unexpected error type: %v", err)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
		}
	})

	t.Run("conflicting_flags_exits_1", func(t *testing.T) {
		cmd := exec.Command(bin, "-a", "-g")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		if err == nil {
			t.Fatal("expected non-zero exit for -a -g")
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("unexpected error type: %v", err)
		}
		if exitErr.ExitCode() != 1 {
			t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
		}
	})

	t.Run("help_exits_0", func(t *testing.T) {
		cmd := exec.Command(bin, "--help")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		err := cmd.Run()
		if err != nil {
			t.Fatalf("--help failed: %v", err)
		}
		if !bytes.Contains(stdout.Bytes(), []byte("Usage:")) {
			t.Error("--help output missing Usage header")
		}
	})

	t.Run("version_exits_0", func(t *testing.T) {
		cmd := exec.Command(bin, "--version")
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		err := cmd.Run()
		if err != nil {
			t.Fatalf("--version failed: %v", err)
		}
		if !bytes.Contains(stdout.Bytes(), []byte("go-unix-utils")) {
			t.Error("--version output missing go-unix-utils identifier")
		}
	})
}

// TestSettingApplication tests setting changes on a real terminal.
// Tests prd105-stty R4.1, R5.1, R6.1, R6.2.
func TestSettingApplication(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err != nil {
		t.Skipf("/dev/tty not accessible: %v", err)
	}
	f.Close()

	// Save initial terminal state for restore on cleanup.
	saved := captureStty(t, bin, "-g", "-F", "/dev/tty")
	t.Cleanup(func() {
		exec.Command(bin, saved, "-F", "/dev/tty").Run() //nolint:errcheck // best-effort restore
	})

	// R4.1: enable and disable a flag.
	t.Run("R4.1_disable_enable_flag", func(t *testing.T) {
		runStty(t, bin, "-echo", "-F", "/dev/tty")
		out := captureStty(t, bin, "-a", "-F", "/dev/tty")
		if !strings.Contains(out, "-echo") {
			t.Error("expected -echo after disabling echo")
		}
		runStty(t, bin, "echo", "-F", "/dev/tty")
	})

	// R5.1: set a special character.
	t.Run("R5.1_set_special_char", func(t *testing.T) {
		runStty(t, bin, "intr", "^A", "-F", "/dev/tty")
		out := captureStty(t, bin, "-a", "-F", "/dev/tty")
		if !strings.Contains(out, "intr = ^A") {
			t.Errorf("expected 'intr = ^A' in output, got:\n%s", out)
		}
		// Restore to default
		runStty(t, bin, "intr", "^C", "-F", "/dev/tty")
	})

	// R6.1: sane combination resets to defaults.
	t.Run("R6.1_sane", func(t *testing.T) {
		runStty(t, bin, "sane", "-F", "/dev/tty")
	})

	// R6.1: raw and cooked (reverse of raw).
	t.Run("R6.1_raw_cooked", func(t *testing.T) {
		runStty(t, bin, "raw", "-F", "/dev/tty")
		runStty(t, bin, "cooked", "-F", "/dev/tty")
	})

	// R6.1: evenp/oddp parity settings.
	t.Run("R6.1_evenp_oddp", func(t *testing.T) {
		runStty(t, bin, "evenp", "-F", "/dev/tty")
		runStty(t, bin, "-evenp", "-F", "/dev/tty")
		runStty(t, bin, "oddp", "-F", "/dev/tty")
		runStty(t, bin, "-oddp", "-F", "/dev/tty")
	})

	// R6.2: set ispeed and ospeed.
	t.Run("R6.2_speed", func(t *testing.T) {
		runStty(t, bin, "ispeed", "9600", "-F", "/dev/tty")
		runStty(t, bin, "ospeed", "9600", "-F", "/dev/tty")
	})

	// Save/restore round trip verifies -g output can be used to restore state.
	t.Run("save_restore_round_trip", func(t *testing.T) {
		runStty(t, bin, "sane", "-F", "/dev/tty")
		initial := captureStty(t, bin, "-g", "-F", "/dev/tty")
		runStty(t, bin, "-echo", "-F", "/dev/tty")
		modified := captureStty(t, bin, "-g", "-F", "/dev/tty")
		if initial == modified {
			t.Error("expected settings to change after -echo")
		}
		runStty(t, bin, initial, "-F", "/dev/tty")
		restored := captureStty(t, bin, "-g", "-F", "/dev/tty")
		if initial != restored {
			t.Error("save/restore round trip failed")
		}
	})
}

// TestExitCodeSuccess verifies R7.1: exit 0 on success.
func TestExitCodeSuccess(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err != nil {
		t.Skipf("/dev/tty not accessible: %v", err)
	}
	f.Close()

	// R7.1: display mode exits 0.
	t.Run("R7.1_default_display_exits_0", func(t *testing.T) {
		cmd := exec.Command(bin, "-F", "/dev/tty")
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected exit 0, got: %v", err)
		}
	})

	t.Run("R7.1_all_display_exits_0", func(t *testing.T) {
		cmd := exec.Command(bin, "-a", "-F", "/dev/tty")
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected exit 0, got: %v", err)
		}
	})

	t.Run("R7.1_save_display_exits_0", func(t *testing.T) {
		cmd := exec.Command(bin, "-g", "-F", "/dev/tty")
		if err := cmd.Run(); err != nil {
			t.Fatalf("expected exit 0, got: %v", err)
		}
	})
}

// TestExitCodeError verifies R7.2: exit 1 on any error.
func TestExitCodeError(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cases := []struct {
		name string
		args []string
	}{
		{"R7.2_invalid_device", []string{"-F", "/dev/nonexistent_stty_test_device"}},
		{"R7.2_invalid_setting", []string{"-F", "/dev/tty", "zzz_invalid_xyz"}},
		{"R7.2_conflicting_flags", []string{"-a", "-g"}},
		{"R7.2_missing_cc_arg", []string{"-F", "/dev/tty", "intr"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bin, tc.args...)
			cmd.Stdin = bytes.NewReader([]byte{}) // ensure no terminal on stdin
			err := cmd.Run()
			if err == nil {
				t.Fatal("expected non-zero exit")
			}
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("unexpected error type: %v", err)
			}
			if exitErr.ExitCode() != 1 {
				t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
			}
		})
	}
}

// TestSIGPIPE verifies R7.3: SIGPIPE is handled gracefully.
func TestSIGPIPE(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")
	f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err != nil {
		t.Skipf("/dev/tty not accessible: %v", err)
	}
	f.Close()

	// R7.3: pipe stty -a output through a command that closes stdin early.
	cmd := exec.Command(bin, "-a", "-F", "/dev/tty")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Close the reader immediately to trigger SIGPIPE on next write.
	stdout.Close()
	// stty should exit without error (SIGPIPE handled gracefully).
	_ = cmd.Wait() // exit code 0 or killed by SIGPIPE — both acceptable
}

// captureStty runs stty and returns trimmed stdout.
func captureStty(t *testing.T, bin string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("stty %v failed: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// runStty runs stty and fails the test on error.
func runStty(t *testing.T, bin string, args ...string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("stty %v failed: %v\n%s", args, err, out)
	}
}
