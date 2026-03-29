// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import "testing"

// NormalizeFunc transforms raw output bytes before comparison.
// R1.2: type alias for output normalization functions.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case comparing a Go binary
// against a GNU reference binary.
//
// R1.1: fields cover arguments, stdin, environment, working directory,
// expected exit code, output normalizers, and expected file-system state.
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

// TimestampNormalizer replaces timestamps in output with a fixed placeholder
// so time-dependent output can be compared deterministically.
//
// R1.4: stub — not yet implemented.
var TimestampNormalizer NormalizeFunc

// RunDiffTests executes each DiffTest against both the Go binary and the
// reference binary, comparing stdout, stderr, and exit code.
//
// R1.3: stub — panics until the implementation task fills in the body.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	panic("not implemented")
}

// ComposeNormalizers chains multiple NormalizeFunc values into a single
// NormalizeFunc that applies each in order.
//
// R1.4: stub — panics until the implementation task fills in the body.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	panic("not implemented")
}

// BuildBinary compiles the Go package in dir and returns the path to the
// resulting binary. It calls t.Fatal on build failure.
//
// Required by the differential testing shared protocol in ARCHITECTURE.yaml.
func BuildBinary(t *testing.T, dir string) string {
	panic("not implemented")
}
