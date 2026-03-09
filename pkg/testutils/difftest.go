// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It runs a Go binary and a GNU reference binary side by side with identical inputs
// and compares stdout, stderr, and exit code.
//
// Implements: prd001-testutils R1.1–R1.4, R5.1–R5.2.
package testutils

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
