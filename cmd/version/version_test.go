// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Unit tests for cmd/version against prd011-magefiles R1.
//
// Covers R1.1 (no-args output), R1.2 (default "dev" version),
// R1.4 (--version, -v flags, unrecognized flag rejection, unexpected argument rejection).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// goBinaryPath is the path to the Go version binary built in TestMain.
var goBinaryPath string

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "version-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}

	goBinaryPath = filepath.Join(tmpDir, "version")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building version: %v\n", err)
		os.RemoveAll(tmpDir)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// runVersion executes the version binary with the given args and returns
// stdout, stderr, and the exit code.
func runVersion(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(goBinaryPath, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("unexpected error running version binary: %v", err)
		}
	}

	return stdout, stderr, exitCode
}

// ---------------------------------------------------------------------------
// R1.1, R1.2: No-args invocation prints "dev" and exits 0
// ---------------------------------------------------------------------------

func TestVersion_NoArgs(t *testing.T) {
	stdout, _, exitCode := runVersion(t)

	if exitCode != 0 {
		t.Errorf("exit code: got %d, want 0", exitCode)
	}
	if stdout != "dev\n" {
		t.Errorf("stdout: got %q, want %q", stdout, "dev\n")
	}
}

// ---------------------------------------------------------------------------
// R1.4: --version and -v flags print same output as no-args
// ---------------------------------------------------------------------------

func TestVersion_VersionFlag(t *testing.T) {
	tests := []struct {
		name string
		flag string
	}{
		{"long_flag_version", "--version"},
		{"short_flag_v", "-v"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, exitCode := runVersion(t, tt.flag)

			if exitCode != 0 {
				t.Errorf("exit code: got %d, want 0", exitCode)
			}
			if stdout != "dev\n" {
				t.Errorf("stdout: got %q, want %q", stdout, "dev\n")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// R1.4: Unrecognized flags cause exit 2 with usage on stderr
// ---------------------------------------------------------------------------

func TestVersion_UnrecognizedFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"unknown_long_flag", []string{"--unknown"}},
		{"unknown_short_flag", []string{"-x"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitCode := runVersion(t, tt.args...)

			if exitCode != 2 {
				t.Errorf("exit code: got %d, want 2", exitCode)
			}
			if stdout != "" {
				t.Errorf("stdout: got %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "Usage:") {
				t.Errorf("stderr should contain usage message, got %q", stderr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// R1.4: Unexpected non-flag arguments cause exit 2 with usage on stderr
// ---------------------------------------------------------------------------

func TestVersion_UnexpectedArgument(t *testing.T) {
	stdout, stderr, exitCode := runVersion(t, "foo")

	if exitCode != 2 {
		t.Errorf("exit code: got %d, want 2", exitCode)
	}
	if stdout != "" {
		t.Errorf("stdout: got %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr should contain usage message, got %q", stderr)
	}
}
