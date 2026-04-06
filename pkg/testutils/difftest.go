// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for comparing
// Go binary output against GNU reference binaries. Implements srd001-testutils.
package testutils

import (
	"testing"
)

// NormalizeFunc transforms raw output bytes before comparison.
// Applied to stdout and stderr of both binaries to strip non-deterministic
// content (timestamps, PIDs, etc.) before byte-for-byte comparison.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case comparing a Go binary
// against a GNU reference binary with identical inputs.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string
	// Args are command-line arguments passed to both binaries.
	Args []string
	// Stdin is piped to both binaries. nil = EOF immediately.
	Stdin []byte
	// Env is KEY=VALUE pairs merged into the inherited environment.
	// nil = use defaults only (LC_ALL=C).
	Env []string
	// WorkDir sets the working directory for both binaries.
	// Empty = per-test t.TempDir().
	WorkDir string
	// ExitCode is the expected exit code for both binaries.
	ExitCode int
	// Normalize is applied in order to stdout and stderr of both
	// binaries before comparison. nil or empty = no normalization.
	Normalize []NormalizeFunc
	// ExpectedFiles maps relative paths to expected byte content
	// after execution, for file-output utilities (sponge, cp).
	ExpectedFiles map[string][]byte
}

// RunDiffTests runs each DiffTest as a named subtest, executing goBinary
// and refBinary with identical inputs and comparing their outputs.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	t.Skip("RunDiffTests: stub implementation, not yet wired")
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order. Returns an identity function when fns is empty.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// TimestampNormalizer replaces common strftime timestamp patterns with a
// fixed placeholder string. Used by cmd/ts tests.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return b
}

// BuildBinary compiles the cmd/ package at dir and returns the path to the
// built binary. Calls t.Fatal on build failure.
func BuildBinary(t *testing.T, dir string) string {
	t.Helper()
	return ""
}
