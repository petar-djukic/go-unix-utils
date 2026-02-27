// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils implements the differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares their stdout, stderr, and exit codes byte-for-byte after optional
// normalization.
//
// Implements: prd001-testutils R1–R5
// Architecture: pkg/testutils component (docs/ARCHITECTURE.yaml)
package testutils

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

const (
	// defaultTimeout is the maximum time each binary invocation is allowed to run.
	// A binary that exceeds this limit causes the test to fail with a timeout
	// message (prd001 R2.3).
	defaultTimeout = 10 * time.Second

	// stdinTruncateLen is the maximum number of stdin bytes included in a
	// divergence report. Longer inputs are truncated at this boundary (prd001 R3.5).
	stdinTruncateLen = 256
)

// Normalize is a function that transforms output bytes before comparison.
// Implementations must not modify the input slice in place; they must return a
// new slice when the content changes (prd001 R1.4).
type Normalize func([]byte) []byte

// FileExpectation specifies expected file content that the go binary must produce
// after execution (prd001 R5.1).
type FileExpectation struct {
	// Path is the file path relative to the go binary's working directory.
	Path string
	// ExpectedContent is the expected byte-for-byte file content.
	ExpectedContent []byte
}

// DiffTest defines a single differential test case (prd001 R1.1).
type DiffTest struct {
	// Name is the subtest label used with t.Run (prd001 R3.6).
	Name string

	// Args are the command-line arguments passed to both binaries (prd001 R1.1).
	Args []string

	// Stdin is the content written to both binaries' stdin. When nil, both
	// binaries receive EOF immediately (prd001 R1.2).
	Stdin []byte

	// Env contains environment overrides in "KEY=VALUE" form. When nil, both
	// binaries inherit the test process environment unchanged. When non-nil,
	// each override is appended to the inherited environment (prd001 R1.3).
	Env []string

	// ExpectedExitCode documents the expected exit code for both binaries
	// (prd001 R1.1). RunDiffTests compares the two binaries' actual exit codes
	// against each other; see R3.4.
	ExpectedExitCode int

	// Normalize is the ordered sequence of normalization functions applied to
	// both binaries' stdout and stderr before comparison. When nil or empty, no
	// normalization is applied (prd001 R1.4, R4.3).
	Normalize []Normalize

	// WorkDir is the working directory for both binaries. When empty,
	// RunDiffTests creates a separate per-test temporary directory for each
	// binary via t.TempDir() (prd001 R2.5, D5).
	WorkDir string

	// FileExpectations specifies expected file content in the go binary's
	// working directory after execution (prd001 R5.1).
	FileExpectations []FileExpectation
}

// RunDiffTests runs each element of tests as a named subtest via t.Run. Each
// subtest executes goBinary and refBinary with identical inputs and compares
// their stdout, stderr, and exit codes. No output is produced for passing
// tests (prd001 AC1, R2.4, R3.6).
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		tc := tc // capture range variable for subtest
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// runDiffTest executes one differential test case.
func runDiffTest(t *testing.T, goBinary, refBinary string, tc DiffTest) {
	t.Helper()

	refWorkDir, goWorkDir := resolveWorkDirs(t, tc.WorkDir)
	env := resolveEnv(tc.Env)

	refOut, refErr, refCode, refTimedOut := runBinary(refBinary, tc.Args, tc.Stdin, env, refWorkDir)
	goOut, goErr, goCode, goTimedOut := runBinary(goBinary, tc.Args, tc.Stdin, env, goWorkDir)

	if refTimedOut {
		t.Errorf("reference binary %q timed out after %s (args: %v)", refBinary, defaultTimeout, tc.Args)
		return
	}
	if goTimedOut {
		t.Errorf("go binary %q timed out after %s (args: %v)", goBinary, defaultTimeout, tc.Args)
		return
	}

	normalizer := Compose(tc.Normalize...)
	normRefOut := normalizer(refOut)
	normGoOut := normalizer(goOut)
	normRefErr := normalizer(refErr)
	normGoErr := normalizer(goErr)

	if !bytes.Equal(normRefOut, normGoOut) || !bytes.Equal(normRefErr, normGoErr) || refCode != goCode {
		t.Errorf("output divergence:\n%s",
			formatDivergence(tc.Args, tc.Stdin, refOut, goOut, refErr, goErr, refCode, goCode))
	}

	for _, fe := range tc.FileExpectations {
		checkFileExpectation(t, goWorkDir, fe)
	}
}

// resolveWorkDirs returns working directories for the reference and go binaries.
// When workDir is non-empty, both binaries share it. When empty, each binary
// receives an isolated per-test temporary directory (prd001 R2.5, D5).
func resolveWorkDirs(t *testing.T, workDir string) (refDir, goDir string) {
	t.Helper()
	if workDir != "" {
		return workDir, workDir
	}
	return t.TempDir(), t.TempDir()
}

// resolveEnv returns the environment slice for binary invocation. When overrides
// is nil, returns nil so exec.Cmd inherits the parent process environment. When
// non-nil, appends each override to the current process environment (prd001 R1.3).
func resolveEnv(overrides []string) []string {
	if overrides == nil {
		return nil
	}
	return append(os.Environ(), overrides...)
}

// runBinary executes binary with the given parameters and returns its captured
// stdout, stderr, exit code, and whether it exceeded the default timeout
// (prd001 R2.2, R2.3, D4).
func runBinary(binary string, args []string, stdin []byte, env []string, workDir string) (stdout, stderr []byte, exitCode int, timedOut bool) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	if env != nil {
		cmd.Env = env
	}
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil, -1, true
	}

	code := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
	}

	return outBuf.Bytes(), errBuf.Bytes(), code, false
}

// checkFileExpectation reads the file at fe.Path relative to workDir and
// compares its content against fe.ExpectedContent byte-for-byte (prd001 R5.2).
func checkFileExpectation(t *testing.T, workDir string, fe FileExpectation) {
	t.Helper()

	path := fe.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(workDir, fe.Path)
	}

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("file expectation: cannot read %q: %v", fe.Path, err)
		return
	}

	if !bytes.Equal(actual, fe.ExpectedContent) {
		t.Errorf("file content mismatch:\npath:     %q\nexpected: %q\nactual:   %q",
			fe.Path, fe.ExpectedContent, actual)
	}
}

// formatDivergence returns a human-readable divergence report including args,
// stdin truncated to stdinTruncateLen bytes, both binaries' stdout and stderr,
// and both exit codes (prd001 R3.5).
func formatDivergence(args []string, stdin, refOut, goOut, refErr, goErr []byte, refCode, goCode int) string {
	stdinDisplay := stdin
	if len(stdinDisplay) > stdinTruncateLen {
		stdinDisplay = stdinDisplay[:stdinTruncateLen]
	}
	return fmt.Sprintf(
		"args:       %v\n"+
			"stdin:      %q\n"+
			"ref stdout: %q\n"+
			"go  stdout: %q\n"+
			"ref stderr: %q\n"+
			"go  stderr: %q\n"+
			"ref exit:   %d\n"+
			"go  exit:   %d",
		args,
		stdinDisplay,
		refOut,
		goOut,
		refErr,
		goErr,
		refCode,
		goCode,
	)
}
