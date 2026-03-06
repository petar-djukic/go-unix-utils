// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness (prd001-testutils).
package testutils

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// NormalizeFunc transforms output bytes before comparison.
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

// BuildBinary compiles the Go package at dir and returns the path to the binary.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	absDir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	binName := filepath.Base(absDir)
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, binName)
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = absDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build %s: %v\n%s", dir, err, out)
	}
	return binPath
}

// RunDiffTests runs each test case against both binaries and compares outputs.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			goOut, goErr, goExit := runBinary(goBinary, tc)
			refOut, refErr, refExit := runBinary(refBinary, tc)

			// Apply normalizers.
			for _, fn := range tc.Normalize {
				goOut = fn(goOut)
				refOut = fn(refOut)
				goErr = fn(goErr)
				refErr = fn(refErr)
			}

			if goExit != refExit {
				t.Errorf("exit code: go=%d ref=%d\nargs: %v\ngo stdout: %q\nref stdout: %q\ngo stderr: %q\nref stderr: %q",
					goExit, refExit, tc.Args, goOut, refOut, goErr, refErr)
			}
			if !bytes.Equal(goOut, refOut) {
				t.Errorf("stdout mismatch:\nargs: %v\ngo:  %q\nref: %q", tc.Args, goOut, refOut)
			}
			if !bytes.Equal(goErr, refErr) {
				t.Errorf("stderr mismatch:\nargs: %v\ngo:  %q\nref: %q", tc.Args, goErr, refErr)
			}
		})
	}
}

// runBinary executes a binary with the given test case parameters.
func runBinary(bin string, tc DiffTest) (stdout, stderr []byte, exitCode int) {
	cmd := exec.Command(bin, tc.Args...)
	if tc.Stdin != nil {
		cmd.Stdin = bytes.NewReader(tc.Stdin)
	}
	if tc.WorkDir != "" {
		cmd.Dir = tc.WorkDir
	}
	cmd.Env = append(os.Environ(), tc.Env...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			fmt.Fprintf(&errBuf, "exec error: %v", err)
		}
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}
