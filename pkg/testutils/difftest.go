// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code byte-for-byte.
//
// Implements: prd001-testutils (R1–R4).
package testutils

import "time"

// defaultTimeout is the maximum duration each binary invocation is allowed
// before the test fails. R2.3: default 10 seconds.
const defaultTimeout = 10 * time.Second

// maxStdinReport is the maximum number of stdin bytes shown in failure messages.
// R3.5: truncated to 256 bytes if longer.
const maxStdinReport = 256

// lcAllKey is the environment variable key for locale override.
const lcAllKey = "LC_ALL"

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: type alias so callers can use plain func([]byte) []byte values.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case. R1.1–R1.4.
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
