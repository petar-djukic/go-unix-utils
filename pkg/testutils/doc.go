// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for comparing
// Go utility binaries against GNU reference binaries.
//
// Implements prd001-testutils: DiffTest defines test cases with arguments,
// stdin, environment, and expected outputs. RunDiffTests executes each case
// against both a Go binary and a reference binary, comparing stdout, stderr,
// and exit code. BuildBinary compiles a cmd/ package into a temporary binary
// for testing.
//
// NormalizeFunc hooks allow tests to mask non-deterministic output (timestamps,
// PIDs) before comparison. TimestampNormalizer is a built-in normalizer for
// common strftime patterns. ComposeNormalizers chains multiple normalizers.
package testutils
