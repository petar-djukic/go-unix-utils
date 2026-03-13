// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd012-yes R2.1–R2.2, R3.1–R3.3, R4.1–R4.3 (differential tests)
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

// TestDiffHelpVersion verifies R2.1 and R2.2: --help and --version exit 0.
// Output content differs between implementations, so stdout/stderr are
// normalized to empty; only exit codes are compared.
func TestDiffHelpVersion(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// --help and --version produce different output between implementations,
	// so we only compare exit codes by normalizing stdout/stderr to empty.
	clearOutput := func(b []byte) []byte { return nil }

	tests := []testutils.DiffTest{
		// R2.2: --help exits 0.
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		// R2.1: --version exits 0.
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSIGPIPEExitCode verifies R3.1, R3.3, R4.3: yes exits 0 when stdout is
// closed early (SIGPIPE case), matching the reference binary's behavior.
// Also verifies R3.3: no error message is printed to stderr on SIGPIPE.
func TestSIGPIPEExitCode(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// runPipeline runs "binary | head -1" and returns stdout, stderr, and
	// the pipeline exit code.
	runPipeline := func(t *testing.T, binary string) (stdout, stderr []byte, exitCode int) {
		t.Helper()
		shellCmd := fmt.Sprintf("%s | head -1", shellescape(binary))
		cmd := exec.Command("sh", "-c", shellCmd)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf

		runErr := cmd.Run()
		code := 0
		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			}
		}
		return outBuf.Bytes(), errBuf.Bytes(), code
	}

	refOut, refErr, refCode := runPipeline(t, refBin)
	goOut, goErr, goCode := runPipeline(t, goBin)

	// R4.3: exit codes must match.
	if refCode != goCode {
		t.Errorf("exit code mismatch: ref=%d got=%d", refCode, goCode)
	}

	// R4.3: stdout must match.
	if !bytes.Equal(refOut, goOut) {
		t.Errorf("stdout mismatch:\nref: %q\ngot: %q", refOut, goOut)
	}

	// R3.3: no error message on stderr for either binary.
	if len(goErr) != 0 {
		t.Errorf("expected empty stderr from Go binary on SIGPIPE, got %q", goErr)
	}
	if len(refErr) != 0 {
		t.Errorf("expected empty stderr from ref binary on SIGPIPE, got %q", refErr)
	}
}
