// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It implements the DiffTest struct and NormalizeFunc type used by cmd/ packages
// to verify Go binary output against Homebrew GNU reference binaries.
//
// Implements: prd001-testutils R1.1-R1.4, R2.1-R2.6, R3.3-R3.6, R4.2-R4.4
package testutils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// NormalizeFunc is a type alias for a function that normalizes output bytes
// before comparison. Using a type alias (=) allows plain func literals to be
// assigned without conversion.
//
// R1.2: NormalizeFunc type alias.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. The harness runs both the
// Go binary and the reference binary with the same inputs and compares their
// outputs after applying any Normalize functions.
//
// R1.1: DiffTest struct with all required fields.
type DiffTest struct {
	// Name is the human-readable test case identifier used in t.Run.
	Name string

	// Args are the command-line arguments passed to both binaries.
	Args []string

	// Stdin is the input written to both binaries' standard input. A nil
	// value means no stdin.
	Stdin []byte

	// Env is the list of environment variable assignments (KEY=VALUE) added
	// to the subprocess environment. When nil, the harness inherits the
	// current process environment.
	Env []string

	// WorkDir is the working directory for both subprocess invocations. An
	// empty string means the test's temporary directory is used.
	WorkDir string

	// ExitCode is the expected exit code that the reference binary produces.
	// The harness compares the Go binary exit code against this value.
	//
	// D3: ExitCode is the expected reference exit code.
	ExitCode int

	// Normalize is an ordered list of functions applied to both stdout and
	// stderr before byte-for-byte comparison. Use this to strip timestamps,
	// PIDs, or other non-deterministic output.
	Normalize []NormalizeFunc

	// ExpectedFiles maps relative file paths to expected byte contents. After
	// the Go binary exits, the harness verifies that each listed file exists
	// in WorkDir and matches the expected content.
	ExpectedFiles map[string][]byte
}

// maxStdinDisplay is the maximum number of stdin bytes included in failure
// messages. Longer input is truncated to this length.
//
// R3.5: stdin content is truncated to 256 bytes in failure messages.
const maxStdinDisplay = 256

// RunDiffTests iterates over tests and runs each case as t.Run(tc.Name, ...).
// For each case it executes goBinary and refBinary with identical inputs,
// applies any NormalizeFunc entries to both stdout and stderr, and reports
// divergence via t.Errorf so all cases run regardless of failures.
//
// R2.1-R2.6, R3.3-R3.6: RunDiffTests execution engine with stderr normalization
// and stdin-inclusive divergence reporting.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			// R2.5, R4.2: Always create a fresh per-test temp dir. When WorkDir
			// is non-empty it is treated as a source directory whose contents are
			// copied into the temp dir so each test case gets an isolated copy.
			workDir := t.TempDir()
			if tc.WorkDir != "" {
				if err := copyDir(tc.WorkDir, workDir); err != nil {
					t.Fatalf("RunDiffTests: failed to copy WorkDir %q: %v", tc.WorkDir, err)
				}
			}
			tc.WorkDir = workDir

			refStdout, refStderr, refCode := runBinary(t, refBinary, tc)
			goStdout, goStderr, goCode := runBinary(t, goBinary, tc)

			// R3.1, R3.3: Apply each NormalizeFunc to both stdout and stderr before comparison.
			for _, fn := range tc.Normalize {
				refStdout = fn(refStdout)
				goStdout = fn(goStdout)
				refStderr = fn(refStderr)
				goStderr = fn(goStderr)
			}

			// R3.5: Truncate stdin display to maxStdinDisplay bytes for failure messages.
			stdinDisplay := tc.Stdin
			if len(stdinDisplay) > maxStdinDisplay {
				stdinDisplay = stdinDisplay[:maxStdinDisplay]
			}

			// R3.4, R3.5: Report exit code divergence with args and stdin.
			if refCode != goCode {
				t.Errorf("exit code mismatch for %q args=%v stdin=%q: ref=%d got=%d",
					tc.Name, tc.Args, stdinDisplay, refCode, goCode)
			}
			// R3.5: Report stdout divergence with args, stdin, and both exit codes.
			if !bytes.Equal(refStdout, goStdout) {
				t.Errorf("stdout mismatch for %q args=%v stdin=%q ref_exit=%d go_exit=%d:\n%s",
					tc.Name, tc.Args, stdinDisplay, refCode, goCode,
					formatDiff("ref stdout", refStdout, "got stdout", goStdout))
			}
			// R3.3, R3.5: Report stderr divergence with args, stdin, and both exit codes.
			if !bytes.Equal(refStderr, goStderr) {
				t.Errorf("stderr mismatch for %q args=%v stdin=%q ref_exit=%d go_exit=%d:\n%s",
					tc.Name, tc.Args, stdinDisplay, refCode, goCode,
					formatDiff("ref stderr", refStderr, "got stderr", goStderr))
			}

			// R4.3, R4.4: Check expected file contents in the working directory
			// after both binaries have run.
			checkExpectedFiles(t, tc.WorkDir, tc.ExpectedFiles)
		})
	}
}

// runBinary executes binary with the inputs from tc and returns captured
// stdout, stderr, and exit code. Errors that are not ExitErrors cause t.Fatalf.
func runBinary(t *testing.T, binary string, tc DiffTest) (stdout, stderr []byte, exitCode int) {
	t.Helper()

	cmd := exec.Command(binary, tc.Args...)

	// R2.6: Build the environment with LC_ALL=C as the default, then merge
	// any overrides from tc.Env on top (which may replace LC_ALL).
	cmd.Env = buildEnv(tc.Env)

	// R2.5: WorkDir is always set by RunDiffTests (empty → t.TempDir()).
	cmd.Dir = tc.WorkDir

	if tc.Stdin != nil {
		cmd.Stdin = bytes.NewReader(tc.Stdin)
	}

	// D4: Capture stdout and stderr separately via bytes.Buffer.
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run binary %q: %v", binary, err)
		}
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}

// buildEnv constructs the subprocess environment by starting with the inherited
// process environment, applying LC_ALL=C as a default, then merging any
// KEY=VALUE pairs from overrides on top.
//
// R2.6: LC_ALL=C is applied before overrides so callers can explicitly set a
// different locale by including LC_ALL in their DiffTest.Env slice.
func buildEnv(overrides []string) []string {
	// Seed from the current process environment.
	envMap := make(map[string]string, len(os.Environ())+1)
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		envMap[k] = v
	}

	// R2.6: Apply LC_ALL=C default before merging overrides.
	envMap["LC_ALL"] = "C"

	// Merge caller-supplied overrides (may replace LC_ALL or any other key).
	for _, kv := range overrides {
		k, v, _ := strings.Cut(kv, "=")
		envMap[k] = v
	}

	env := make([]string, 0, len(envMap))
	for k, v := range envMap {
		env = append(env, k+"="+v)
	}
	return env
}

// formatDiff returns a human-readable side-by-side label for two byte slices.
func formatDiff(labelA string, a []byte, labelB string, b []byte) string {
	return fmt.Sprintf("%s: %q\n%s: %q", labelA, a, labelB, b)
}

// copyDir recursively copies the contents of src into dst. dst must already
// exist. Symlinks are not followed; only regular files and directories are
// copied.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("reading dir %q: %w", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if mkErr := os.MkdirAll(dstPath, 0o755); mkErr != nil {
				return fmt.Errorf("creating dir %q: %w", dstPath, mkErr)
			}
			if copyErr := copyDir(srcPath, dstPath); copyErr != nil {
				return copyErr
			}
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return fmt.Errorf("stat %q: %w", srcPath, infoErr)
		}
		data, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			return fmt.Errorf("reading %q: %w", srcPath, readErr)
		}
		if writeErr := os.WriteFile(dstPath, data, info.Mode()); writeErr != nil {
			return fmt.Errorf("writing %q: %w", dstPath, writeErr)
		}
	}
	return nil
}

// checkExpectedFiles reads each entry in expectedFiles from the working
// directory and compares it byte-for-byte against the expected content.
// Missing files and content mismatches are both reported via t.Errorf.
//
// R4.3: Byte-for-byte comparison of each ExpectedFiles entry.
// R4.4: Missing file causes t.Errorf with path and expected content.
func checkExpectedFiles(t *testing.T, workDir string, expectedFiles map[string][]byte) {
	t.Helper()
	for relPath, expected := range expectedFiles {
		actualPath := filepath.Join(workDir, filepath.FromSlash(relPath))
		actual, err := os.ReadFile(actualPath)
		if err != nil {
			if os.IsNotExist(err) {
				t.Errorf("expected file %q does not exist in WorkDir; expected content: %q", relPath, expected)
			} else {
				t.Errorf("reading expected file %q: %v", relPath, err)
			}
			continue
		}
		if !bytes.Equal(expected, actual) {
			t.Errorf("file content mismatch for %q:\nexpected: %q\nactual:   %q", relPath, expected, actual)
		}
	}
}
