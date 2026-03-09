// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for cmd/ packages.
// It executes a Go binary and a reference GNU binary with identical inputs and
// compares stdout, stderr, and exit code.
// Implements prd001-testutils.
package testutils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// NormalizeFunc transforms output bytes before comparison, allowing acceptable
// differences (e.g., timestamps) to be stripped.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case.
type DiffTest struct {
	Name          string
	Args          []string
	Stdin         []byte
	Env           []string
	WorkDir       string
	ExitCode      int
	Normalize     []NormalizeFunc
	ExpectedFiles map[string][]byte
}

// ComposeNormalizers chains multiple NormalizeFunc into one, applying them in order.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// TimestampNormalizer is a placeholder normalizer for timestamp-sensitive output.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return b
}

// BuildBinary compiles the Go package at the given path and returns the path
// to the resulting binary. The binary is placed in a temporary directory that
// is cleaned up when the test finishes.
func BuildBinary(t *testing.T, pkg string) string {
	t.Helper()

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "test-binary")

	// Resolve package path relative to caller's directory.
	absPath, err := filepath.Abs(pkg)
	if err != nil {
		t.Fatalf("resolving package path %q: %v", pkg, err)
	}

	cmd := exec.Command("go", "build", "-o", binPath, absPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("building binary from %q: %v", pkg, err)
	}

	return binPath
}

// RunDiffTests runs each DiffTest by executing both the Go binary and the
// reference binary with the same arguments, stdin, and environment, then
// compares stdout, stderr, and exit code.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			goOut, goErr, goExit := runBinary(t, goBinary, tc)
			refOut, refErr, refExit := runBinary(t, refBinary, tc)

			// Apply normalizers.
			for _, norm := range tc.Normalize {
				goOut = norm(goOut)
				refOut = norm(refOut)
				goErr = norm(goErr)
				refErr = norm(refErr)
			}

			if goExit != refExit {
				t.Errorf("exit code: go=%d ref=%d\ngo stdout: %q\ngo stderr: %q\nref stdout: %q\nref stderr: %q",
					goExit, refExit, goOut, goErr, refOut, refErr)
			}

			if !bytes.Equal(goOut, refOut) {
				t.Errorf("stdout differs:\n  go:  %q\n  ref: %q", goOut, refOut)
			}

			if !bytes.Equal(goErr, refErr) {
				// For stderr, only check that go binary produces output when ref does,
				// since exact error message wording may differ.
				if len(refErr) > 0 && len(goErr) == 0 {
					t.Errorf("stderr: ref produced output but go did not\n  ref stderr: %q", refErr)
				}
				if len(refErr) == 0 && len(goErr) > 0 {
					t.Errorf("stderr: go produced output but ref did not\n  go stderr: %q", goErr)
				}
			}
		})
	}
}

// runBinary executes a binary with the given DiffTest parameters and returns
// stdout, stderr, and exit code.
func runBinary(t *testing.T, binary string, tc DiffTest) (stdout, stderr []byte, exitCode int) {
	t.Helper()

	cmd := exec.Command(binary, tc.Args...)
	if tc.Stdin != nil {
		cmd.Stdin = bytes.NewReader(tc.Stdin)
	}

	// Set LC_ALL=C by default, then overlay test-specific env.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Env = append(cmd.Env, tc.Env...)

	if tc.WorkDir != "" {
		cmd.Dir = tc.WorkDir
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("running %s: %v", filepath.Base(binary), err)
		}
	}

	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}

// WriteTestFile creates a file with the given content in the specified directory.
// Returns the absolute path to the created file.
func WriteTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
	return path
}

// WriteTestFileBytes creates a file with the given byte content in the specified directory.
// Returns the absolute path to the created file.
func WriteTestFileBytes(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
	return path
}

// FormatDiff produces a human-readable diff description between two byte slices.
func FormatDiff(label string, got, want []byte) string {
	return fmt.Sprintf("%s:\n  got:  %q\n  want: %q", label, got, want)
}
