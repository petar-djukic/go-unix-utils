// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, ANSI color output, and column alignment.
// Implements prd003-format.
package format

import "fmt"

// HumanSizeOpts controls the unit base for HumanSize. R3.1.
type HumanSizeOpts struct {
	// Binary selects 1024-based units with suffixes K/M/G/T/P/E when true,
	// and 1000-based SI units with suffixes kB/MB/GB/TB when false.
	Binary bool
}

// binarySuffixes are the suffixes used for 1024-based human-readable sizes.
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are the suffixes used for 1000-based SI human-readable sizes.
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string. R3.1–R3.4.
//
// When opts.Binary is true, base 1024 is used with suffixes K, M, G, T, P, E.
// When opts.Binary is false, base 1000 is used with suffixes kB, MB, GB, TB.
// Zero always returns "0".
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
			return formatSize(val, suffixes[i])
		}
		val /= base
	}
	return formatSize(val, suffixes[len(suffixes)-1])
}

// formatSize formats a value with a suffix, using at most one decimal place. R3.3.
func formatSize(val float64, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%d", int64(val))
	}
	// Show one decimal place only when the value is not an integer.
	if val == float64(int64(val)) {
		return fmt.Sprintf("%d%s", int64(val), suffix)
	}
	return fmt.Sprintf("%.1f%s", val, suffix)
}
