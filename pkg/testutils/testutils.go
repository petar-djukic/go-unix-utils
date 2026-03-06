// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness (prd001-testutils).
package testutils

import (
	"bytes"
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

// BuildBinary compiles the Go package in dir and returns the path to the binary.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "testbin")
	cmd := exec.Command("go", "build", "-o", bin, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build %s: %v\n%s", dir, err, out)
	}
	return bin
}

// RunDiffTests runs each test case against both the Go binary and the reference
// binary, comparing stdout, stderr, and exit code byte-for-byte.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			goOut, goErr, goExit := runCmd(goBinary, tc)
			refOut, refErr, refExit := runCmd(refBinary, tc)
			for _, n := range tc.Normalize {
				goOut = n(goOut)
				refOut = n(refOut)
				goErr = n(goErr)
				refErr = n(refErr)
			}
			if goExit != refExit {
				t.Errorf("exit code: go=%d ref=%d", goExit, refExit)
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

func runCmd(binary string, tc DiffTest) (stdout, stderr []byte, exitCode int) {
	cmd := exec.Command(binary, tc.Args...)
	if tc.Stdin != nil {
		cmd.Stdin = bytes.NewReader(tc.Stdin)
	}
	cmd.Env = append(os.Environ(), tc.Env...)
	if tc.WorkDir != "" {
		cmd.Dir = tc.WorkDir
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}
