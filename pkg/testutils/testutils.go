// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for comparing
// Go binary output against GNU reference binaries.
// Implements prd001-testutils.
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

type runResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// BuildBinary compiles the Go package at pkg and returns the path to the
// resulting binary. The binary is placed in t.TempDir().
func BuildBinary(t *testing.T, pkg string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "binary")
	cmd := exec.Command("go", "build", "-o", bin, pkg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("BuildBinary: go build failed: %v\n%s", err, out)
	}
	return bin
}

// RunDiffTests executes each test case against both goBinary and refBinary,
// comparing exit code, stdout, and stderr. Normalizers are applied before
// comparison.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			goRes := runBinary(t, goBinary, tc)
			refRes := runBinary(t, refBinary, tc)

			if goRes.exitCode != refRes.exitCode {
				t.Errorf("exit code mismatch: go=%d ref=%d", goRes.exitCode, refRes.exitCode)
			}

			goOut := goRes.stdout
			refOut := refRes.stdout
			goErr := goRes.stderr
			refErr := refRes.stderr
			for _, n := range tc.Normalize {
				goOut = n(goOut)
				refOut = n(refOut)
				goErr = n(goErr)
				refErr = n(refErr)
			}

			if !bytes.Equal(goOut, refOut) {
				t.Errorf("stdout mismatch:\n  go:  %q\n  ref: %q", goOut, refOut)
			}
			if !bytes.Equal(goErr, refErr) {
				t.Errorf("stderr mismatch:\n  go:  %q\n  ref: %q", goErr, refErr)
			}
		})
	}
}

// ComposeNormalizers chains multiple NormalizeFuncs into one.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

func runBinary(t *testing.T, binary string, tc DiffTest) runResult {
	t.Helper()
	cmd := exec.Command(binary, tc.Args...)

	if len(tc.Stdin) > 0 {
		cmd.Stdin = bytes.NewReader(tc.Stdin)
	}

	env := os.Environ()
	env = append(env, tc.Env...)
	cmd.Env = env

	if tc.WorkDir != "" {
		cmd.Dir = tc.WorkDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}

	return runResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// TimestampNormalizer strips timestamps from output for comparison.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return b
}
