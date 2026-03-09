// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// column alignment, ANSI color output, and human-readable unit conversion.
// Implements prd003-format R1–R4.
package format

import "fmt"

// HumanSizeOpts controls the unit system for HumanSize.
type HumanSizeOpts struct {
	// Binary selects 1024-based units (K, M, G, T, P, E) when true,
	// and 1000-based SI units (kB, MB, GB, TB) when false. R3.1, R3.2.
	Binary bool
}

// binarySuffixes are 1024-based unit labels matching GNU coreutils --human-readable.
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are 1000-based unit labels matching GNU coreutils --si.
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string.
// R3.1: Binary=true uses 1024-based units; Binary=false uses 1000-based SI units.
// R3.3: formats with one decimal place when the value is not an integer at the chosen unit.
// R3.4: returns "0" for zero bytes.
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

// formatValue renders a numeric value with a unit suffix.
// Values < 10 get one decimal place; values >= 10 get one decimal place;
// values >= 100 get zero decimal places. Plain integers (no suffix) are
// rendered without decimals. R3.3.
func formatValue(val float64, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%d", int64(val))
	}
	if val >= 100 {
		return fmt.Sprintf("%d%s", int64(val), suffix)
	}
	if val >= 10 {
		return fmt.Sprintf("%.1f%s", val, suffix)
	}
	return fmt.Sprintf("%.1f%s", val, suffix)
}
