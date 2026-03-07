// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for verifying Go
// reimplementations of Unix utilities against Homebrew GNU reference binaries.
// Implements prd001-testutils.
package testutils

import (
	"regexp"
)

// NormalizeFunc transforms raw binary output bytes before comparison.
// (prd001-testutils R1.4)
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. Both the Go binary and the
// GNU reference binary are invoked with identical Args, Stdin, Env, and WorkDir.
// (prd001-testutils R1.1)
type DiffTest struct {
	Name          string              // subtest name used with t.Run; required
	Args          []string            // command-line arguments passed to both binaries
	Stdin         []byte              // nil = no stdin (both binaries receive EOF immediately)
	Env           []string            // nil = use defaults only (LC_ALL=C); non-nil = KEY=VALUE pairs merged into inherited environment
	WorkDir       string              // empty = per-test t.TempDir(); non-empty = use this directory for both binaries
	ExitCode      int                 // expected exit code for both binaries
	Normalize     []NormalizeFunc     // applied in order to stdout and stderr of both binaries before comparison; nil or empty = no normalization
	ExpectedFiles map[string][]byte   // optional: path -> expected byte content after execution, for file-output utilities (sponge, cp)
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions left to right. (prd001-testutils R4.4)
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// timestampPatterns matches common strftime-formatted timestamps that appear in
// utility output. The patterns cover formats like "Feb 19 12:34:56",
// "2026-02-19 12:34:56", "12:34:56", and ISO 8601 variants.
var timestampPatterns = []*regexp.Regexp{
	// "Mon DD HH:MM:SS" (e.g., "Feb 19 12:34:56")
	regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
	// "YYYY-MM-DD HH:MM:SS" (e.g., "2026-02-19 12:34:56")
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`),
	// "HH:MM:SS" standalone (e.g., "12:34:56")
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
}

const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string so differential tests pass despite wall-clock differences.
// (prd001-testutils R4.2)
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	result := b
	for _, pat := range timestampPatterns {
		result = pat.ReplaceAll(result, []byte(timestampPlaceholder))
	}
	return result
}
