// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// column alignment, ANSI color output, and human-readable size conversion.
//
// Implements prd003-format (R1, R2, R3).
package format

import "fmt"

// HumanSizeOpts configures human-readable size formatting.
type HumanSizeOpts struct {
	// Binary selects the unit base. true uses 1024-based units with suffixes
	// K, M, G, T, P, E. false uses SI 1000-based units with suffixes
	// kB, MB, GB, TB.
	Binary bool
}

var (
	binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}
	siSuffixes     = []string{"", "kB", "MB", "GB", "TB"}
)

// HumanSize converts a byte count to a human-readable string matching GNU
// coreutils human_readable() behavior. Values below the first unit threshold
// are returned as plain integers. Values at or above a unit threshold are
// formatted with one decimal place when under 10, or zero decimal places
// otherwise. Zero always returns "0".
//
// prd003-format R3.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	if bytes == 0 {
		return "0"
	}

	base := 1000.0
	suffixes := siSuffixes
	if opts.Binary {
		base = 1024.0
		suffixes = binarySuffixes
	}

	val := float64(bytes)
	for i := 0; i < len(suffixes)-1; i++ {
		if val < base {
			if i == 0 {
				return fmt.Sprintf("%d", bytes)
			}
			return formatWithSuffix(val, suffixes[i])
		}
		val /= base
	}
	return formatWithSuffix(val, suffixes[len(suffixes)-1])
}

// formatWithSuffix formats a scaled value with its unit suffix. Uses one
// decimal place for values under 10, no decimal place otherwise.
func formatWithSuffix(val float64, suffix string) string {
	if val < 10 {
		return fmt.Sprintf("%.1f%s", val, suffix)
	}
	return fmt.Sprintf("%.0f%s", val, suffix)
}
