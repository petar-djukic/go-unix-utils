// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It implements the DiffTest struct and NormalizeFunc type used by cmd/ packages
// to verify Go binary output against Homebrew GNU reference binaries.
//
// Implements: prd001-testutils R1.1-R1.4
package testutils

// NormalizeFunc is a type alias for a function that normalizes output bytes
// before comparison. Using a type alias (=) allows plain func literals to be
// assigned without conversion.
//
// R1.2: NormalizeFunc type alias.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. The harness runs both the
// Go binary and the reference binary with the same inputs and compares their
// outputs after applying any Normalize functions.
//
// R1.1: DiffTest struct with all required fields.
type DiffTest struct {
	// Name is the human-readable test case identifier used in t.Run.
	Name string

	// Args are the command-line arguments passed to both binaries.
	Args []string

	// Stdin is the input written to both binaries' standard input. A nil
	// value means no stdin.
	Stdin []byte

	// Env is the list of environment variable assignments (KEY=VALUE) added
	// to the subprocess environment. When nil, the harness sets LC_ALL=C.
	Env []string

	// WorkDir is the working directory for both subprocess invocations. An
	// empty string means the test's temporary directory is used.
	WorkDir string

	// ExitCode is the expected exit code that the reference binary produces.
	// The harness compares the Go binary exit code against this value.
	//
	// D3: ExitCode is the expected reference exit code.
	ExitCode int

	// Normalize is an ordered list of functions applied to both stdout and
	// stderr before byte-for-byte comparison. Use this to strip timestamps,
	// PIDs, or other non-deterministic output.
	Normalize []NormalizeFunc

	// ExpectedFiles maps relative file paths to expected byte contents. After
	// the Go binary exits, the harness verifies that each listed file exists
	// in WorkDir and matches the expected content.
	ExpectedFiles map[string][]byte
}
