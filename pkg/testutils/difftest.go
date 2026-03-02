// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for go-unix-utils.
// It executes a Go binary and a reference GNU binary with identical inputs and
// compares stdout, stderr, and exit code byte-for-byte.
//
// Implements: prd001-testutils (R1, R2, R3, R4, R5)
package testutils

import (
	"regexp"
	"testing"
	"time"
)

// DefaultTimeout is the maximum duration for each binary invocation.
// Tests may override this value before calling RunDiffTests.
// (prd001-testutils R2.3)
var DefaultTimeout = 10 * time.Second

// NormalizeFunc transforms raw output bytes before comparison. Applied to stdout
// and stderr of both binaries when present in DiffTest.Normalize.
// (prd001-testutils R1.4)
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. Both the Go binary under
// test and the reference GNU binary are invoked with identical Args, Stdin, and
// Env. (prd001-testutils R1.1)
type DiffTest struct {
	Name          string            // subtest name used with t.Run; required
	Args          []string          // command-line arguments passed to both binaries
	Stdin         []byte            // nil = no stdin (EOF immediately); non-nil = piped to both binaries
	Env           []string          // nil = defaults only (LC_ALL=C); non-nil = KEY=VALUE pairs merged into environment
	WorkDir       string            // empty = per-test t.TempDir(); non-empty = use this directory for both binaries
	ExitCode      int               // expected exit code for both binaries
	Normalize     []NormalizeFunc   // applied in order to stdout and stderr before comparison; nil = no normalization
	ExpectedFiles map[string][]byte // optional: path -> expected byte content after execution
}

// diffResult holds captured output from a single binary execution.
type diffResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// RunDiffTests executes each DiffTest as a named subtest, running both the Go
// binary and the reference binary with identical inputs and comparing outputs.
// (prd001-testutils R2.1)
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			runSingleDiffTest(t, goBinary, refBinary, tc)
		})
	}
}

// TimestampNormalizer replaces common strftime timestamp patterns with the fixed
// placeholder "<TIMESTAMP>". Used by cmd/ts tests to normalize wall-clock
// differences. (prd001-testutils R4.2)
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	s := string(b)
	for _, re := range timestampPatterns {
		s = re.ReplaceAllString(s, "<TIMESTAMP>")
	}
	return []byte(s)
}

// timestampPatterns matches common strftime-formatted timestamps, ordered from
// most specific to least specific so longer matches are replaced first.
var timestampPatterns = []*regexp.Regexp{
	// "%b %d %H:%M:%S" — e.g., "Feb 19 12:34:56"
	regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s\d{2}:\d{2}:\d{2}`),
	// "%Y-%m-%d %H:%M:%S" — e.g., "2026-02-19 12:34:56"
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s\d{2}:\d{2}:\d{2}`),
	// "%H:%M:%S" — e.g., "12:34:56"
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order. (prd001-testutils R4.4)
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}
