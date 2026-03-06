// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness used by cmd/
// packages to verify Go utility implementations against reference GNU binaries.
// Each test case executes two binaries with identical inputs and compares the
// Go binary's outputs against expected values.
//
// Implements: prd001-testutils
// Architecture: docs/ARCHITECTURE.yaml § pkg/testutils
package testutils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// DiffTest defines a single differential test case. Both the reference binary
// (refBin) and the Go binary (goBin) are executed with identical inputs; the
// Go binary's outputs are compared against the expected Want* values.
//
// Implements prd001-testutils R1.
type DiffTest struct {
	// Args are the command-line arguments passed to both binaries.
	Args []string

	// Stdin is content piped to stdin of both binaries.
	// nil means both binaries receive EOF immediately.
	Stdin []byte

	// Env contains additional KEY=VALUE environment variable pairs merged
	// into the inherited process environment. Matching keys are overridden;
	// new keys are added.
	Env []string

	// WantExit is the expected exit code of the Go binary.
	WantExit int

	// WantStdout is the expected stdout output of the Go binary.
	WantStdout []byte

	// WantStderr is the expected stderr output of the Go binary.
	WantStderr []byte

	// NormStdout, when non-nil, is applied to the actual stdout before
	// comparison, enabling normalization of acceptable differences (e.g.,
	// stripping trailing newlines or timestamps).
	NormStdout func([]byte) []byte

	// NormStderr, when non-nil, is applied to the actual stderr before
	// comparison.
	NormStderr func([]byte) []byte
}

// Run executes refBin and goBin with identical inputs and reports divergence
// between the Go binary's outputs and the expected Want* values. t.Errorf is
// used (not t.Fatalf) so all three mismatch kinds — exit code, stdout, stderr
// — are reported in a single invocation without halting at the first failure.
//
// Implements prd001-testutils R2, R3.
func (tc DiffTest) Run(t *testing.T, refBin, goBin string) {
	t.Helper()

	env := append(os.Environ(), tc.Env...)

	// Execute the reference binary. Its output is not compared against
	// Want*; it is run to confirm the reference binary is reachable.
	if _, err := runBinary(refBin, tc.Args, tc.Stdin, env); err != nil {
		t.Fatalf("reference binary %s: %v", refBin, err)
	}

	// Execute the Go binary and capture its outputs.
	res, err := runBinary(goBin, tc.Args, tc.Stdin, env)
	if err != nil {
		t.Fatalf("go binary %s: %v", goBin, err)
	}

	gotStdout := res.stdout
	gotStderr := res.stderr
	if tc.NormStdout != nil {
		gotStdout = tc.NormStdout(gotStdout)
	}
	if tc.NormStderr != nil {
		gotStderr = tc.NormStderr(gotStderr)
	}

	// R3: report each mismatch independently so all failures are visible.
	if res.exitCode != tc.WantExit {
		t.Errorf("%s: exit code = %d, want %d", goBin, res.exitCode, tc.WantExit)
	}
	if !bytes.Equal(gotStdout, tc.WantStdout) {
		t.Errorf("%s: stdout = %q, want %q", goBin, gotStdout, tc.WantStdout)
	}
	if !bytes.Equal(gotStderr, tc.WantStderr) {
		t.Errorf("%s: stderr = %q, want %q", goBin, gotStderr, tc.WantStderr)
	}
}

// binaryResult holds the captured output of a single binary execution.
type binaryResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBinary executes the binary at path with the given arguments, stdin, and
// environment and returns the captured stdout, stderr, and exit code.
// It returns an error only when the binary cannot be started; a non-zero exit
// code is captured in the result, not treated as an error.
func runBinary(path string, args []string, stdin []byte, env []string) (binaryResult, error) {
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(path, args...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Env = env

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError) //nolint:errorlint
		if !ok {
			return binaryResult{}, fmt.Errorf("starting %s: %w", path, err)
		}
		exitCode = exitErr.ExitCode()
	}
	return binaryResult{
		stdout:   outBuf.Bytes(),
		stderr:   errBuf.Bytes(),
		exitCode: exitCode,
	}, nil
}
