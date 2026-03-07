// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, ANSI color output, padding, and column layout.
// Implements prd003-format.
package format

import "fmt"

// HumanSizeOpts controls the unit system for HumanSize.
type HumanSizeOpts struct {
	// Binary selects 1024-based units with Ki/Mi/Gi suffixes when true,
	// and 1000-based SI units with K/M/G suffixes when false.
	Binary bool
}

var (
	// R3.2: binary (base 1024) suffixes.
	binarySuffixes = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei"}
	// R3.2: SI (base 1000) suffixes.
	siSuffixes = []string{"", "K", "M", "G", "T", "P", "E"}
)

// HumanSize converts a byte count to a human-readable string. When opts.Binary
// is true, uses base-1024 with Ki/Mi/Gi suffixes. When false, uses base-1000
// with K/M/G suffixes. (prd003-format R3.1–R3.6)
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	// R3.4: zero is always "0".
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
	for i, suffix := range suffixes {
		next := base
		if i == len(suffixes)-1 || val < next {
			// R3.3: at most one decimal place when not integer at this unit.
			if suffix == "" {
				return fmt.Sprintf("%d", bytes)
			}
			if val == float64(int64(val)) {
				return fmt.Sprintf("%.1f%s", val, suffix)
			}
			return fmt.Sprintf("%.1f%s", val, suffix)
		}
		val /= base
	}

	// Unreachable for reasonable inputs; largest suffix absorbs everything.
	last := suffixes[len(suffixes)-1]
	return fmt.Sprintf("%.1f%s", val, last)
}
