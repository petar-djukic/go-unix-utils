// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for comparing
// Go binaries against GNU reference binaries (prd001-testutils).
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

// BuildBinary compiles the Go package at dir and returns the path to the binary.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "testbin")
	cmd := exec.Command("go", "build", "-o", bin, dir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %s\n%s", err, out)
	}
	return bin
}

// RunDiffTests runs each test case against both binaries and compares outputs.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			goOut, goErr, goExit := runBinary(goBinary, tc)
			refOut, refErr, refExit := runBinary(refBinary, tc)

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
				t.Errorf("stdout differs:\ngo:  %q\nref: %q", goOut, refOut)
			}
			if !bytes.Equal(goErr, refErr) {
				t.Errorf("stderr differs:\ngo:  %q\nref: %q", goErr, refErr)
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

// TimestampNormalizer replaces common timestamp patterns with a placeholder.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return b // placeholder for utilities that need timestamp normalization
}

func runBinary(bin string, tc DiffTest) (stdout, stderr []byte, exitCode int) {
	cmd := exec.Command(bin, tc.Args...)
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
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}
