// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sizeparse parses human-readable size strings (e.g. "10K", "5M") into
// byte counts. Implements srd087-sizeparse.
package sizeparse

// ParseOptions configures size string parsing behavior.
// R1: AllowSign permits leading +/- signs; DefaultUnit scales bare numbers.
type ParseOptions struct {
	AllowSign   bool
	DefaultUnit int64
}

// Parse parses a human-readable size string into a byte count.
// R2: uses default options (no sign, unit=1).
func Parse(s string) (int64, error) {
	return 0, nil
}

// ParseWithOptions parses a human-readable size string with configurable options.
// R3: respects AllowSign and DefaultUnit from opts.
func ParseWithOptions(s string, opts ParseOptions) (int64, error) {
	return 0, nil
}
