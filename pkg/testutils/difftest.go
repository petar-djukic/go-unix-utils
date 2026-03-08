// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code byte-for-byte.
//
// Implements prd001-testutils (R1, R3, R4).
package testutils

import "regexp"

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: type alias for normalization functions used in DiffTest.Normalize.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case.
// R1.1: all fields are documented; the zero value is a valid test case.
type DiffTest struct {
	Name          string            // subtest name used with t.Run; required
	Args          []string          // command-line arguments passed to both binaries
	Stdin         []byte            // nil = no stdin (both binaries receive EOF immediately)
	Env           []string          // nil = use defaults only (LC_ALL=C); non-nil = KEY=VALUE pairs merged into inherited environment
	WorkDir       string            // empty = per-test t.TempDir(); non-empty = use this directory for both binaries
	ExitCode      int               // expected exit code for both binaries
	Normalize     []NormalizeFunc   // applied in order to stdout and stderr of both binaries before comparison; nil or empty = no normalization
	ExpectedFiles map[string][]byte // optional: path -> expected byte content after execution, for file-output utilities (sponge, cp)
}

// ComposeNormalizers chains multiple NormalizeFunc values into a single function
// that applies them in order.
// R4.4: convenience for cmd/ test files combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// timestampPlaceholder is the fixed string that replaces timestamp patterns.
const timestampPlaceholder = "<TIMESTAMP>"

// timestampPatterns matches common strftime-formatted timestamps.
var timestampPatterns = []*regexp.Regexp{
	// ISO 8601: 2026-03-07T12:34:56 or 2026-03-07 12:34:56
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`),
	// Syslog: Feb 19 12:34:56 or Feb  1 12:34:56
	regexp.MustCompile(`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}`),
	// Unix epoch with fractional seconds: 1709812345.123456
	regexp.MustCompile(`\d{10}\.\d+`),
	// Bare time HH:MM:SS
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
}

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder so time-dependent outputs can be compared.
// R4.2: used by cmd/ts tests.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	result := make([]byte, len(data))
	copy(result, data)
	for _, pat := range timestampPatterns {
		result = pat.ReplaceAll(result, []byte(timestampPlaceholder))
	}
	return result
}
