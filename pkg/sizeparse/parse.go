// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sizeparse parses size strings with unit suffixes (K, M, G, etc.)
// into byte counts. Implements srd087-sizeparse.
package sizeparse

// ParseOptions configures the behavior of ParseWithOptions.
type ParseOptions struct {
	// AllowSign permits +/- prefix on size strings.
	AllowSign bool
	// DefaultUnit is the multiplier when no suffix is given (default 1).
	DefaultUnit int64
}

// Parse parses a size string consisting of a decimal integer and an optional
// unit suffix, returning the size in bytes. Returns an error for invalid input.
func Parse(s string) (int64, error) {
	return 0, nil
}

// ParseWithOptions parses a size string with configurable behavior controlled
// by opts. See ParseOptions for available options.
func ParseWithOptions(s string, opts ParseOptions) (int64, error) {
	return 0, nil
}
