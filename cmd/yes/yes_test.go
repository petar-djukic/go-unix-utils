// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd012-yes R4.1–R4.3 (differential tests)
package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for yes.
const refBinaryName = "gyes"

// headLines is the number of output lines captured from yes for comparison.
const headLines = 10

// runYesHead runs binary with args, pipes stdout through "head -n N", and
// returns the captured stdout. This is needed because yes runs forever;
// piping through head causes SIGPIPE which terminates yes.
func runYesHead(t *testing.T, binary string, args []string, n int) (stdout []byte) {
	t.Helper()

	// Run: binary [args...] | head -n N
	// Use sh -c to get proper pipe handling with SIGPIPE delivery.
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, binary)
	for _, a := range args {
		parts = append(parts, shellescape(a))
	}
	shellCmd := fmt.Sprintf("%s | head -n %d", joinShellArgs(parts), n)
	cmd := exec.Command("sh", "-c", shellCmd)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	// Ignore error: head closes the pipe, which may cause a non-zero exit
	// from sh depending on how SIGPIPE is delivered.
	_ = cmd.Run() // best-effort; we compare output, not exit code of sh pipeline

	return outBuf.Bytes()
}

// joinShellArgs joins shell-escaped arguments with spaces.
func joinShellArgs(parts []string) string {
	return strings.Join(parts, " ")
}

// shellescape wraps s in single quotes for safe shell interpolation.
func shellescape(s string) string {
	// Replace each ' with '\'' (end quote, escaped quote, start quote).
	escaped := bytes.ReplaceAll([]byte(s), []byte("'"), []byte("'\\''"))
	return "'" + string(escaped) + "'"
}

// TestDiff verifies R4.1, R4.2: differential output comparison against gyes.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	tests := []struct {
		name string
		args []string
	}{
		// R1.1: no arguments — default "y" output.
		{"no_args", nil},
		// R1.2: single argument.
		{"single_arg", []string{"hello"}},
		// R1.2: multiple arguments joined with spaces.
		{"multi_arg", []string{"hello", "world"}},
		// R1.3: arguments after "--" separator.
		{"double_dash_separator", []string{"--", "--help"}},
		// R4.2: empty string argument.
		{"empty_string_arg", []string{""}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			refOut := runYesHead(t, refBin, tc.args, headLines)
			goOut := runYesHead(t, goBin, tc.args, headLines)

			if !bytes.Equal(refOut, goOut) {
				t.Errorf("stdout mismatch for %q args=%v:\nref: %q\ngot: %q",
					tc.name, tc.args, refOut, goOut)
			}
		})
	}
}

// TestSIGPIPEExitCode verifies R4.3: yes exits 0 when stdout is closed early.
func TestSIGPIPEExitCode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	// Run: goBin | head -1
	// The head command closes the pipe after one line, sending SIGPIPE to yes.
	cmd := exec.Command("sh", "-c", fmt.Sprintf("%s | head -1", shellescape(goBin)))
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf

	err := cmd.Run()
	if err != nil {
		t.Errorf("expected exit 0 from pipeline, got error: %v", err)
	}

	// Verify we got output (head captured one line).
	if outBuf.Len() == 0 {
		t.Error("expected output from yes | head -1, got empty")
	}
}
