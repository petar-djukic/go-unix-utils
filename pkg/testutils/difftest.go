// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for comparing
// Go utility binaries against GNU reference binaries.
//
// Implements prd001-testutils R1.1–R1.4: core types and function signatures.
package testutils

import (
	"testing"
)

// NormalizeFunc transforms raw output bytes before comparison.
// R1.2: type alias for output normalization functions.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case that runs a Go binary and
// a reference binary with identical inputs and compares their outputs.
//
// R1.1: all fields match the prd001-testutils contract.
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

// RunDiffTests executes each DiffTest against both goBinary and refBinary,
// comparing stdout, stderr, and exit code.
//
// R1.3: stub implementation — skips until the execution logic is implemented.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Skip("not implemented")
}

// ComposeNormalizers chains multiple NormalizeFunc values into a single
// NormalizeFunc that applies them in order.
//
// R1.4: stub implementation.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	panic("not implemented")
}

// TimestampNormalizer replaces timestamp patterns in output bytes with a
// fixed placeholder so that time-dependent output can be compared.
//
// R1.4: stub implementation.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	panic("not implemented")
}
