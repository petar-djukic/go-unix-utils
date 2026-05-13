// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// It executes a Go binary and a GNU reference binary with identical inputs and
// compares stdout, stderr, and exit code.
// Implements srd001-testutils.
package testutils

import (
	"testing"
)

// NormalizeFunc transforms raw output bytes before comparison.
// R1.4: named type alias for normalization functions.
type NormalizeFunc = func([]byte) []byte

// DiffTest defines a single differential test case.
// R1.1: fields match the package contract.
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
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return b
}

// RunDiffTests runs each DiffTest as a named subtest, executing goBinary and
// refBinary with identical inputs and comparing their outputs.
// R2.1: accepts paths to Go and reference binaries plus test cases.
func RunDiffTests(t *testing.T, goBinary, refBinary string, tests []DiffTest) {
	t.Fatal("not implemented")
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order.
// R4.4: convenience for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// BuildBinary compiles the cmd/ package in dir and returns the path to the
// built binary. It calls t.Fatal if the build fails.
// D4: included per the differential testing shared protocol.
func BuildBinary(t *testing.T, dir string) string {
	t.Fatal("not implemented")
	return ""
}
