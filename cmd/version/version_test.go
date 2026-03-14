// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/version per prd059-version R1.5.
// Covers all PRD acceptance criteria at the binary level: output format
// (R1.1), dev-build fallback (R1.2), unknown-flag rejection (R1.4), and
// parseable version output usable by other cmd/ packages (R1.5).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// versionBin holds the path to the compiled cmd/version binary, built once
// by TestMain before any test runs.
var versionBin string

// TestMain compiles cmd/version once into a temporary directory shared by all
// tests in this package. Using a single build avoids redundant compilation on
// every test function while still exercising the real binary.
func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "cmd-version-test-*")
	if err != nil {
		panic("TestMain: creating temp dir: " + err.Error())
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // best-effort cleanup; error ignored

	binPath := filepath.Join(tmpDir, "version")
	out, buildErr := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
	if buildErr != nil {
		panic("TestMain: building cmd/version: " + string(out))
	}
	versionBin = binPath

	os.Exit(m.Run())
}

// runVersion executes the version binary with args and returns captured
// stdout, stderr, and exit code.
func runVersion(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(versionBin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("runVersion: executing binary: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// TestVersion_NoArgs verifies R1.1 / AC1: invoking with no arguments prints a
// version line to stdout, writes nothing to stderr, and exits 0.
func TestVersion_NoArgs(t *testing.T) {
	t.Parallel()
	stdout, stderr, exitCode := runVersion(t)

	if exitCode != 0 {
		t.Errorf("exit code: want 0, got %d", exitCode)
	}
	if stderr != "" {
		t.Errorf("stderr: want empty, got %q", stderr)
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Errorf("stdout: want trailing newline, got %q", stdout)
	}
	line := strings.TrimRight(stdout, "\n")
	if line == "" {
		t.Errorf("stdout: want non-empty version line, got empty string")
	}
}

// TestVersion_OutputFormat verifies R1.1 and R1.5 / AC1, AC5: the output is
// exactly one line of the form "go-unix-utils <version>" where <version> is
// non-empty. This confirms the version string is structured and parseable by
// other cmd/ packages that invoke the binary to retrieve the version.
func TestVersion_OutputFormat(t *testing.T) {
	t.Parallel()
	stdout, _, _ := runVersion(t)

	// Exactly one newline-terminated line with no extra whitespace.
	if strings.Count(stdout, "\n") != 1 {
		t.Fatalf("stdout: want exactly one line, got %q", stdout)
	}
	line := strings.TrimRight(stdout, "\n")

	parts := strings.SplitN(line, " ", 2)
	if len(parts) != 2 {
		t.Fatalf("stdout %q: want two space-separated fields (name version), got %d field(s)", line, len(parts))
	}
	if parts[0] != "go-unix-utils" {
		t.Errorf("binary name field: want %q, got %q", "go-unix-utils", parts[0])
	}
	if parts[1] == "" {
		t.Errorf("version field: want non-empty string, got empty")
	}
}

// TestVersion_DevBuild verifies R1.2 / AC4: a development build (compiled
// without -ldflags) reports a non-empty fallback version string. The exact
// fallback value is implementation-defined; the test verifies only that it is
// non-empty so the format remains parseable.
func TestVersion_DevBuild(t *testing.T) {
	t.Parallel()
	stdout, _, exitCode := runVersion(t)

	if exitCode != 0 {
		t.Errorf("exit code: want 0 for dev build, got %d", exitCode)
	}
	line := strings.TrimRight(stdout, "\n")
	parts := strings.SplitN(line, " ", 2)
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		t.Errorf("version field in dev build: want non-empty fallback, got output %q", stdout)
	}
}

// TestVersion_UnknownFlag verifies R1.4 / AC3: an unrecognized flag causes the
// binary to write a usage message to stderr and exit with a non-zero code. The
// exact exit code is implementation-defined; the test verifies only that it is
// non-zero and that stderr contains a usage indication.
func TestVersion_UnknownFlag(t *testing.T) {
	t.Parallel()
	stdout, stderr, exitCode := runVersion(t, "--bogus")

	if exitCode == 0 {
		t.Errorf("exit code: want non-zero for unknown flag, got 0")
	}
	if stdout != "" {
		t.Errorf("stdout: want empty for unknown flag, got %q", stdout)
	}
	if !strings.Contains(strings.ToLower(stderr), "usage") {
		t.Errorf("stderr: want usage message containing %q, got %q", "usage", stderr)
	}
}
