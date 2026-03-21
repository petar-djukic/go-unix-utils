// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R3: human-readable size formatting.
package format

import "fmt"

// HumanSizeOpts controls the unit system for HumanSize output.
type HumanSizeOpts struct {
	// Binary selects 1024-based units (K, M, G, T, P, E) when true,
	// and SI 1000-based units (kB, MB, GB, TB) when false.
	Binary bool
}

// R3.2: suffixes for binary (1024-based) mode.
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// R3.2: suffixes for SI (1000-based) mode.
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize formats a byte count as a human-readable string matching
// GNU coreutils conventions. Implements prd003-format R3.1-R3.4.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	// R3.4: zero returns "0" regardless of mode.
	if bytes == 0 {
		return "0"
	}

	base, suffixes := selectUnits(opts)
	value := float64(bytes)

	for i := 0; i < len(suffixes)-1; i++ {
		if absFloat(value) < base {
			return formatValue(value, suffixes[i])
		}
		value /= base
	}

	return formatValue(value, suffixes[len(suffixes)-1])
}

// selectUnits returns the divisor and suffix list for the given options.
func selectUnits(opts HumanSizeOpts) (float64, []string) {
	if opts.Binary {
		return 1024.0, binarySuffixes
	}
	return 1000.0, siSuffixes
}

// formatValue formats a size value with at most one decimal place.
// R3.3: use one decimal when the value is not an integer.
func formatValue(value float64, suffix string) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d%s", int64(value), suffix)
	}
	return fmt.Sprintf("%.1f%s", value, suffix)
}

// absFloat returns the absolute value of f.
func absFloat(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
