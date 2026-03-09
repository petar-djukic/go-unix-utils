// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It runs a Go binary and a GNU reference binary side by side with identical inputs
// and compares stdout, stderr, and exit code.
//
// Implements: prd001-testutils R1.1–R1.4, R4.1–R4.4.
package testutils

import (
	"regexp"
)

// NormalizeFunc transforms binary output before comparison to handle acceptable
// differences like timestamps. R1.4.
type NormalizeFunc = func([]byte) []byte

// DiffTest represents a single differential test case. R1.1.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string
	// Args are command-line arguments passed to both binaries.
	Args []string
	// Stdin is piped to both binaries. nil = EOF immediately; empty slice = open then close.
	Stdin []byte
	// Env is KEY=VALUE pairs merged into inherited environment. nil = defaults only (LC_ALL=C).
	Env []string
	// WorkDir is the working directory for both binaries. Empty = per-test t.TempDir().
	WorkDir string
	// ExitCode is the expected exit code for both binaries.
	ExitCode int
	// Normalize functions applied in order to stdout and stderr before comparison.
	Normalize []NormalizeFunc
	// ExpectedFiles maps relative paths to expected byte content after execution.
	ExpectedFiles map[string][]byte
}

// applyNormalizers applies each NormalizeFunc in order to the data. R4.1, R4.3.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given functions
// in order. R4.4.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// timestampPattern matches common strftime-formatted timestamps. R4.2.
var timestampPattern = regexp.MustCompile(
	// Mon DD HH:MM:SS (e.g., "Feb 19 12:34:56")
	`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` +
		`|` +
		// YYYY-MM-DD HH:MM:SS
		`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}` +
		`|` +
		// HH:MM:SS
		`\d{2}:\d{2}:\d{2}`,
)

// timestampPlaceholder is the fixed string that replaces timestamps.
const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string. R4.2.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	return timestampPattern.ReplaceAll(data, []byte(timestampPlaceholder))
}
