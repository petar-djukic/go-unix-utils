// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for verifying
// Go binary implementations against GNU reference binaries. Implements
// prd001-testutils R1, R2, R3, R4, R5.
package testutils

import (
	"regexp"
)

// DiffTest defines a single differential test case. Each test executes both
// a Go binary and a GNU reference binary with identical inputs and compares
// their outputs. (prd001-testutils R1.1)
type DiffTest struct {
	Name          string                // subtest name used with t.Run; required
	Args          []string              // command-line arguments passed to both binaries
	Stdin         []byte                // nil = no stdin (both binaries receive EOF immediately)
	Env           []string              // nil = inherit env with LC_ALL=C; non-nil = KEY=VALUE pairs merged into inherited env
	WorkDir       string                // empty = per-test t.TempDir(); non-empty = use this directory for both binaries
	ExitCode      int                   // expected exit code for both binaries
	Normalize     []NormalizeFunc       // applied in order to stdout and stderr before comparison; nil = no normalization
	ExpectedFiles map[string][]byte     // optional: path -> expected byte content after execution
}

// NormalizeFunc transforms raw output bytes before comparison. Used to strip
// or replace non-deterministic fields (timestamps, PIDs, paths) so that
// differential tests pass despite expected variation. (prd001-testutils R1.2)
type NormalizeFunc = func([]byte) []byte

// ComposeNormalizers returns a single NormalizeFunc that applies fns in order
// (left to right). Returns nil when fns is empty. (prd001-testutils R1.3, R4.4)
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	if len(fns) == 0 {
		return nil
	}
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// timestampPatterns matches common strftime-formatted timestamps:
//   - "Mon DD HH:MM:SS" (e.g., "Feb 19 12:34:56")
//   - "YYYY-MM-DD HH:MM:SS" (e.g., "2026-02-19 12:34:56")
//   - "HH:MM:SS" standalone (e.g., "12:34:56")
//   - "Mon DD HH:MM:SS YYYY" (e.g., "Feb 19 12:34:56 2026")
var timestampPatterns = []*regexp.Regexp{
	// YYYY-MM-DD HH:MM:SS with optional fractional seconds
	regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(\.\d+)?`),
	// Mon DD HH:MM:SS YYYY (syslog with year)
	regexp.MustCompile(`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2} \d{4}`),
	// Mon DD HH:MM:SS (syslog format)
	regexp.MustCompile(`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}`),
	// HH:MM:SS with optional fractional seconds (standalone time)
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}(\.\d+)?`),
}

const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime-formatted timestamps with a
// fixed placeholder string for comparison. Used by cmd/ts tests to handle
// wall-clock differences between binary executions. (prd001-testutils R1.4, R4.2)
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	result := b
	for _, pat := range timestampPatterns {
		result = pat.ReplaceAll(result, []byte(timestampPlaceholder))
	}
	return result
}
