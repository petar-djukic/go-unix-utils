// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sizeparse provides a size string parser with unit suffixes for
// cmd/ utilities that accept human-readable size arguments.
//
// Implements prd087-sizeparse: R1 (size parsing), R2 (extended parsing
// options), R3 (constraints).
package sizeparse

// ParseOptions configures size string parsing behavior.
//
// R2.1 (prd087): AllowSign enables +/- prefix parsing. DefaultUnit sets the
// multiplier when no suffix is given (default 1).
type ParseOptions struct {
	AllowSign   bool
	DefaultUnit int64
}

// Parse parses a size string consisting of a decimal integer and an optional
// unit suffix, returning the size in bytes.
//
// R1.1 (prd087): parses integer with optional unit suffix.
func Parse(s string) (int64, error) {
	return parseSizeString(s, ParseOptions{})
}

// ParseWithOptions parses a size string with configurable behavior controlled
// by opts.
//
// R2.1 (prd087): supports AllowSign and DefaultUnit options.
func ParseWithOptions(s string, opts ParseOptions) (int64, error) {
	return parseSizeString(s, opts)
}
