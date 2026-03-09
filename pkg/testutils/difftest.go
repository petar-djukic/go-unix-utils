// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for verifying
// Go binary implementations against GNU reference binaries. Implements
// prd001-testutils R1, R2, R3, R4, R5.
//
// R3.5: Divergence reporting includes args, stdin (truncated to 256 bytes),
// reference stdout/stderr, Go stdout/stderr, and both exit codes.
// R3.6: Each DiffTest runs as a named subtest via t.Run for identifiable
// failures in go test output.
package testutils

// DiffTest defines a single differential test case. Each test executes both
// a Go binary and a GNU reference binary with identical inputs and compares
// their outputs. (prd001-testutils R1.1)
type DiffTest struct {
	Name          string            // subtest name used with t.Run; required
	Args          []string          // command-line arguments passed to both binaries
	Stdin         []byte            // nil = no stdin (both binaries receive EOF immediately)
	Env           []string          // nil = inherit env with LC_ALL=C; non-nil = KEY=VALUE pairs merged into inherited env
	WorkDir       string            // empty = per-test t.TempDir(); non-empty = use this directory for both binaries
	ExitCode      int               // expected exit code for both binaries
	Normalize     []NormalizeFunc   // applied in order to stdout and stderr before comparison; nil = no normalization
	ExpectedFiles map[string][]byte // optional: path -> expected byte content after execution
}

// NormalizeFunc transforms raw output bytes before comparison. Used to strip
// or replace non-deterministic fields (timestamps, PIDs, paths) so that
// differential tests pass despite expected variation. (prd001-testutils R1.2)
type NormalizeFunc = func([]byte) []byte
