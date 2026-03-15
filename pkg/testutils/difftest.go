// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

// NormalizeFunc is a type alias for functions that transform raw output bytes
// before comparison. Using a type alias (=) allows bare func literals to be
// assigned without conversion.
//
// R1.4: Named type alias in package testutils.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. Each test runs both the Go
// binary under test and the GNU reference binary with identical Args, Stdin, and
// Env, then compares their outputs.
//
// R1.1: DiffTest struct with all required fields.
type DiffTest struct {
	// Name is the subtest name used with t.Run; required.
	Name string

	// Args are command-line arguments passed to both binaries.
	Args []string

	// Stdin is fed to both binaries on stdin. nil means both binaries receive
	// EOF immediately. An empty non-nil slice ([]byte{}) also produces no bytes
	// but is semantically distinct (open stdin that is immediately closed).
	// R1.2.
	Stdin []byte

	// Env contains KEY=VALUE pairs merged into the inherited environment.
	// nil means use defaults only (LC_ALL=C); non-nil pairs override or extend
	// the inherited environment. R1.3.
	Env []string

	// WorkDir sets the working directory for both binaries. Empty means the
	// harness creates a per-test t.TempDir().
	WorkDir string

	// ExitCode is the expected exit code for both binaries.
	ExitCode int

	// Normalize contains functions applied in order to stdout and stderr of
	// both binaries before comparison. nil or empty means no normalization.
	Normalize []NormalizeFunc

	// ExpectedFiles maps relative paths to expected byte content after
	// execution, for file-output utilities (sponge, cp).
	ExpectedFiles map[string][]byte
}
