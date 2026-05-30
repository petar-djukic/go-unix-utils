// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code.
// Implements srd001-testutils.
package testutils

import "testing"

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: named type alias for normalization functions.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case.
//
// R1.1: Name identifies the subtest (used with t.Run). Args holds command-line
// arguments passed to both binaries.
//
// R1.2: Stdin nil means both binaries receive EOF immediately. An empty non-nil
// slice ([]byte{}) also produces no bytes but represents an open stdin that is
// immediately closed.
//
// R1.3: Env nil means both binaries inherit the test process environment with
// LC_ALL=C applied as a default override (R2.6). When non-nil, KEY=VALUE pairs
// are merged into the inherited environment: matching keys are overridden, new
// keys are added.
//
// R4.1, R4.3: Normalize holds a slice of NormalizeFunc applied in order to
// stdout and stderr of both binaries before comparison. Nil or empty means no
// normalization.
//
// R5.1: ExpectedFiles maps relative paths to expected byte content after
// execution, for file-output utilities.
type DiffTest struct {
	Name          string
	Args          []string
	Stdin         []byte
	Env           []string
	WorkDir       string
	ExitCode      int
	Normalize     []NormalizeFunc
	ExpectedFiles map[string][]byte
}

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string.
// R4.2: built-in normalizer for cmd/ts tests.
var TimestampNormalizer NormalizeFunc = func([]byte) []byte {
	panic("not implemented")
}

// RunDiffTests runs each DiffTest as a named subtest, executing goBinary and
// refBinary with identical inputs and comparing their outputs.
// R2.1, R2.2, R2.3, R2.4, R2.5, R2.6, R3.1, R3.2, R3.3, R3.4, R3.5, R3.6.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	panic("not implemented")
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order.
// R4.4: convenience for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	panic("not implemented")
}
