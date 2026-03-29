// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

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
