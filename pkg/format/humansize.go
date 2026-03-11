// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities:
// column alignment, ANSI color output, and human-readable unit conversion.
//
// Implements: prd003-format R3.1–R3.4 (human-readable size conversion).
package format

import "fmt"

// HumanSizeOpts controls the unit scale used by HumanSize.
// prd003-format R3.1.
type HumanSizeOpts struct {
	// Binary selects 1024-based binary prefixes (K/M/G/T/P/E) when true,
	// or 1000-based SI prefixes (kB/MB/GB/TB) when false.
	Binary bool
}

var (
	binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}
	siSuffixes     = []string{"", "kB", "MB", "GB", "TB"}
)

// HumanSize converts a raw byte count to a human-readable string.
//
// When opts.Binary is true, the divisor is 1024 and suffixes are K/M/G/T/P/E.
// When opts.Binary is false, the divisor is 1000 and suffixes are kB/MB/GB/TB.
// Returns "0" for a zero byte count regardless of unit mode.
// Values below the first threshold are returned as a bare integer.
// All other values are formatted with exactly one decimal place.
//
// prd003-format R3.1–R3.4.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	// R3.4: zero always returns "0".
	if bytes == 0 {
		return "0"
	}

	var base float64
	var suffixes []string
	if opts.Binary {
		// R3.2: binary mode — 1024-based with suffixes K/M/G/T/P/E.
		base = 1024
		suffixes = binarySuffixes
	} else {
		// R3.2: SI mode — 1000-based with suffixes kB/MB/GB/TB.
		base = 1000
		suffixes = siSuffixes
	}

	value := float64(bytes)
	idx := 0
	for idx < len(suffixes)-1 && value >= base {
		value /= base
		idx++
	}

	// Bare bytes: no suffix, format as integer.
	if idx == 0 {
		return fmt.Sprintf("%d", int64(value))
	}

	// R3.3: format with exactly one decimal place.
	return fmt.Sprintf("%.1f%s", value, suffixes[idx])
}
