// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness that executes a Go
// binary and a reference GNU binary side by side and compares stdout, stderr,
// and exit code. Implements prd001-testutils.
package testutils

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
)

// NormalizeFunc transforms output bytes before comparison, allowing
// controlled differences (e.g., timestamps) to be masked.
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

// timestampPatterns matches common timestamp formats for normalization.
var timestampPatterns = []*regexp.Regexp{
	// Syslog: "Jan  5 14:30:00" or "Jan 05 14:30:00"
	regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
	// ISO-8601: "2024-01-05T14:30:00" with optional subseconds and Z
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z?`),
	// HH:MM:SS with optional subseconds
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}(\.\d+)?`),
	// Epoch or seconds with microseconds
	regexp.MustCompile(`\d+\.\d{6}`),
	// Relative age strings: "5d3h45m12s ago"
	regexp.MustCompile(`(\d+[dhms])+\s+ago`),
}

// TimestampNormalizer replaces common timestamp patterns in output with
// the fixed placeholder "TIMESTAMP" so that wall-clock differences do
// not cause test failures in differential tests.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	result := data
	for _, pat := range timestampPatterns {
		result = pat.ReplaceAll(result, []byte("TIMESTAMP"))
	}
	return result
}

// ComposeNormalizers chains multiple NormalizeFuncs into a single function,
// applying them in order.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// BuildBinary compiles the Go package at dir and returns the path to the
// resulting binary. The binary is placed in t.TempDir().
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	tmpDir := t.TempDir()
	binName := filepath.Base(dir)
	if binName == "." {
		// Resolve the current working directory to get a meaningful name.
		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("BuildBinary: cannot get working directory: %v", err)
		}
		binName = filepath.Base(wd)
	}
	binPath := filepath.Join(tmpDir, binName)
	cmd := exec.Command("go", "build", "-o", binPath, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build %s failed: %v\n%s", dir, err, out)
	}
	return binPath
}

// RunDiffTests runs each DiffTest against both goBinary and refBinary,
// comparing exit code, stdout, and stderr after applying normalizers.
// The harness sets LC_ALL=C by default.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			goOut, goErr, goExit := runBinary(t, goBinary, tc)
			refOut, refErr, refExit := runBinary(t, refBinary, tc)

			// Apply normalizers to stdout and stderr.
			for _, norm := range tc.Normalize {
				goOut = norm(goOut)
				refOut = norm(refOut)
				goErr = norm(goErr)
				refErr = norm(refErr)
			}

			if goExit != refExit {
				t.Errorf("exit code differs: go=%d ref=%d", goExit, refExit)
			}
			if !bytes.Equal(goOut, refOut) {
				t.Errorf("stdout differs:\n  go:  %q\n  ref: %q", goOut, refOut)
			}
			if !bytes.Equal(goErr, refErr) {
				t.Errorf("stderr differs:\n  go:  %q\n  ref: %q", goErr, refErr)
			}
		})
	}
}

// runBinary executes a binary with the given DiffTest inputs and returns
// stdout, stderr, and exit code.
func runBinary(t *testing.T, binPath string, tc DiffTest) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	cmd := exec.Command(binPath, tc.Args...)
	cmd.Stdin = bytes.NewReader(tc.Stdin)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	// Set LC_ALL=C by default, then apply test-specific env vars.
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Env = append(cmd.Env, tc.Env...)

	if tc.WorkDir != "" {
		cmd.Dir = tc.WorkDir
	}

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("runBinary %s: unexpected error: %v", binPath, err)
		}
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}
