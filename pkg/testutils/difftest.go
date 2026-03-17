// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code.
//
// Implements prd001-testutils R1.1-R1.4.
package testutils

// NormalizeFunc transforms raw output bytes before comparison. Applied to both
// the Go binary and the reference binary outputs so that non-deterministic
// fields (timestamps, PIDs) do not cause false divergence.
//
// R1.4: Named type alias for func([]byte) []byte.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. Both the Go binary and the
// GNU reference binary are invoked with identical Args, Stdin, and Env. The
// harness compares their stdout, stderr, and exit code after applying any
// normalization functions.
//
// R1.1: DiffTest struct with all specified fields.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string

	// Args are command-line arguments passed to both binaries.
	Args []string

	// Stdin is piped to both binaries. nil means both receive EOF immediately.
	// An empty non-nil slice ([]byte{}) also produces no bytes but is
	// semantically distinct (open stdin that is immediately closed).
	// R1.2.
	Stdin []byte

	// Env contains KEY=VALUE pairs merged into the inherited environment.
	// nil means use defaults only (LC_ALL=C). Non-nil pairs override or
	// extend the inherited environment.
	// R1.3.
	Env []string

	// WorkDir sets the working directory for both binaries. Empty means
	// per-test t.TempDir().
	WorkDir string

	// ExitCode is the expected exit code for both binaries.
	ExitCode int

	// Normalize contains functions applied in order to stdout and stderr of
	// both binaries before comparison. nil or empty means no normalization.
	// R1.4.
	Normalize []NormalizeFunc

	// ExpectedFiles maps relative paths to expected byte content after
	// execution. Used for file-output utilities (sponge, cp).
	ExpectedFiles map[string][]byte
}
