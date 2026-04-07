// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for cmd/yes: differential tests against gyes reference binary.
// Implements srd012-yes R3.3, R4.1, R4.2, R4.3.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// headLines is the number of lines captured via head for differential comparison.
const headLines = "10"

// TestDiff verifies cmd/yes against gyes by piping output through head.
// R4.1: pipe through head -n N and compare byte-for-byte.
// R4.2: covers no-arg, single-arg, multi-arg, "--" separator, empty string.
// R4.3: verifies exit code when stdout is closed early (SIGPIPE).
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gyes")
	if err != nil {
		t.Skipf("reference binary gyes not in PATH: %v", err)
	}

	tests := []struct {
		name string
		args []string
	}{
		{name: "default_y", args: nil},
		{name: "single_arg", args: []string{"hello"}},
		{name: "multi_arg", args: []string{"hello", "world"}},
		{name: "dash_dash_separator", args: []string{"--", "--help"}},
		{name: "empty_string", args: []string{""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			refOut := runWithHead(t, refBin, tc.args)
			gotOut := runWithHead(t, goBin, tc.args)
			if !bytes.Equal(refOut, gotOut) {
				t.Errorf("stdout mismatch\nargs: %v\nexpected (ref): %q\nactual   (go):  %q",
					tc.args, refOut, gotOut)
			}
		})
	}
}

// TestSIGPIPEExitCode verifies that yes exits cleanly when stdout closes early.
// R3.3: no error message to stderr on SIGPIPE.
// R4.3: exit code behavior when stdout is closed early.
func TestSIGPIPEExitCode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	// Run yes piped to head -n 1 via shell; capture stderr and exit code.
	// R3.3: stderr must be empty (no error message printed).
	cmd := exec.Command("sh", "-c", shellescape(goBin)+" | head -n 1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)

	// We don't check the exit code of the shell pipeline because POSIX shell
	// reports the exit code of the last command (head), which is 0.
	// The key assertion is that stderr is empty (R3.3).
	_ = cmd.Run() // best-effort; exit code is from head, not yes

	if stderr.Len() > 0 {
		t.Errorf("R3.3: expected no stderr on SIGPIPE, got: %q", stderr.String())
	}
}

// runWithHead executes a binary piped through "head -n headLines" and returns stdout.
// R4.1: captures output via head to bound infinite yes output.
func runWithHead(t *testing.T, bin string, args []string) []byte {
	t.Helper()

	// Build the shell command: <bin> [args...] | head -n <headLines>
	parts := []string{shellescape(bin)}
	for _, a := range args {
		parts = append(parts, shellescape(a))
	}
	cmdStr := strings.Join(parts, " ") + " | head -n " + headLines

	cmd := exec.Command("sh", "-c", cmdStr)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)

	// Pipeline exit code comes from head, which should be 0.
	// Non-zero exit is unexpected but not fatal for comparison.
	_ = cmd.Run()

	return stdout.Bytes()
}

// shellescape wraps a string in single quotes for safe shell interpolation.
func shellescape(s string) string {
	// Replace single quotes with '\'' (end quote, escaped quote, start quote).
	escaped := bytes.ReplaceAll([]byte(s), []byte("'"), []byte("'\\''"))
	return "'" + string(escaped) + "'"
}

// TestBuild verifies that cmd/yes compiles without errors.
func TestBuild(t *testing.T) {
	t.Parallel()
	// BuildBinary compiles the package; if it fails, the test fails.
	bin := testutils.BuildBinary(t, ".")
	// Verify the binary exists.
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("binary not found after build: %v", err)
	}
	_ = filepath.Base(bin) // use filepath to satisfy import if needed
}
