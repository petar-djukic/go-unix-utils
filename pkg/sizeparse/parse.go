// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sizeparse provides size string parsing with unit suffixes for
// cmd/ utilities that accept human-readable size arguments.
//
// Implements prd087-sizeparse R1.1–R1.4, R2.1–R2.2, R3.1–R3.2:
// ParseOptions, Parse, ParseWithOptions (contract stubs).
package sizeparse

// ParseOptions configures behavior for ParseWithOptions.
//
// R2.1: AllowSign controls whether +/- prefix is accepted.
// DefaultUnit is the multiplier when no suffix is given (default 1).
type ParseOptions struct {
	AllowSign   bool
	DefaultUnit int64
}

// Parse parses a size string consisting of a decimal integer and an
// optional unit suffix, returning the size in bytes.
//
// R1.1: stub implementation — returns 0, nil until execution logic
// is implemented.
func Parse(s string) (int64, error) {
	return 0, nil
}

// ParseWithOptions parses a size string with configurable behavior
// controlled by opts.
//
// R2.1: stub implementation — returns 0, nil until execution logic
// is implemented.
func ParseWithOptions(s string, opts ParseOptions) (int64, error) {
	return 0, nil
}
