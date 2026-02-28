// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across go-unix-utils
// utilities: human-readable size conversion, ANSI color codes for file types,
// and column alignment for tabular output.
//
// Implements: prd003-format R1, R2, R3.
package format

import "fmt"

// HumanSizeOpts configures the behavior of HumanSize.
//
// Implements: prd003-format R3.1.
type HumanSizeOpts struct {
	// Binary selects 1024-based conversion with suffixes K/M/G/T/P/E when true
	// (the default zero value). When false, uses 1000-based SI conversion with
	// suffixes kB/MB/GB/TB.
	Binary bool
}

// binarySuffixes are the unit suffixes for 1024-based human-readable sizes.
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are the unit suffixes for 1000-based SI human-readable sizes.
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string. The conversion
// base and suffixes are selected by opts.Binary.
//
// When opts.Binary is true, uses base 1024 with suffixes K/M/G/T/P/E.
// When opts.Binary is false, uses base 1000 with suffixes kB/MB/GB/TB.
//
// Output uses at most one decimal place when the value is not an integer at
// the chosen unit (e.g., 1536 bytes → "1.5K"). Returns "0" for zero bytes.
//
// Implements: prd003-format R3.1, R3.2, R3.3, R3.4.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	if bytes == 0 {
		return "0"
	}

	var base float64
	var suffixes []string
	if opts.Binary {
		base = 1024
		suffixes = binarySuffixes
	} else {
		base = 1000
		suffixes = siSuffixes
	}

	val := float64(bytes)
	for i := 0; i < len(suffixes)-1; i++ {
		if val < base {
			return formatValue(val, suffixes[i])
		}
		val /= base
	}

	return formatValue(val, suffixes[len(suffixes)-1])
}

// formatValue formats a floating-point value with its suffix. Uses one decimal
// place when the value is less than 10 and not an integer; otherwise uses no
// decimal places.
func formatValue(val float64, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%d", int64(val))
	}
	if val < 10 {
		// Use one decimal place for values under 10.
		s := fmt.Sprintf("%.1f", val)
		// Strip trailing ".0" to produce clean integers (e.g., "1.0K" stays
		// as "1.0K" per GNU coreutils convention — GNU always shows one
		// decimal when the value is < 10).
		return s + suffix
	}
	return fmt.Sprintf("%d", int64(val)) + suffix
}
