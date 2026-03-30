// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/stty against gstty (GNU coreutils).
//
// Tests prd105-stty R1.1, R2.1, R3.1, R3.2.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// progNameNorm normalizes the program name prefix in output so that
// "gstty: " and "stty: " compare as equal.
var progNameNorm = func(data []byte) []byte {
	re := regexp.MustCompile(`(?m)^g?stty: `)
	return re.ReplaceAll(data, []byte("stty: "))
}

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
			Name: "error_no_tty",
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
