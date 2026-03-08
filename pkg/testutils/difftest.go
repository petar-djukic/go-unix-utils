// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for verifying Go
// utility implementations against GNU reference binaries.
//
// Implements prd001-testutils: DiffTest, RunDiffTests, ComposeNormalizers,
// TimestampNormalizer, BuildBinary.
package testutils

import (
	"regexp"
)

// NormalizeFunc transforms raw output bytes before comparison. R1.4.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. R1.1.
type DiffTest struct {
	Name          string            // subtest name used with t.Run; required
	Args          []string          // command-line arguments passed to both binaries
	Stdin         []byte            // nil = no stdin (both binaries receive EOF immediately)
	Env           []string          // nil = use defaults only (LC_ALL=C); non-nil = KEY=VALUE pairs merged into inherited environment
	WorkDir       string            // empty = per-test t.TempDir(); non-empty = use this directory for both binaries
	ExitCode      int               // expected exit code for both binaries
	Normalize     []NormalizeFunc   // applied in order to stdout and stderr of both binaries before comparison; nil or empty = no normalization
	ExpectedFiles map[string][]byte // optional: path -> expected byte content after execution
}

// Compiled regexps for TimestampNormalizer. Applied longest-match-first to
// avoid partial replacement of embedded time components.
var (
	// Mon DD HH:MM:SS (syslog style: "Feb 19 12:34:56")
	reTimestampSyslog = regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`)
	// YYYY-MM-DD HH:MM:SS (ISO style)
	reTimestampISO = regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`)
	// HH:MM:SS (time only)
	reTimestampTime = regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)
)

const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime timestamp patterns with a stable
// placeholder for comparison. R4.2.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	placeholder := []byte(timestampPlaceholder)
	result := reTimestampSyslog.ReplaceAll(b, placeholder)
	result = reTimestampISO.ReplaceAll(result, placeholder)
	result = reTimestampTime.ReplaceAll(result, placeholder)
	return result
}

// ComposeNormalizers chains multiple NormalizeFunc values into a single function
// that applies them in order. R4.4.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}
