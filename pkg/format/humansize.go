// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R1.1-R1.4: HumanSizeOpts struct and HumanSize function.

package format

import "fmt"

// HumanSizeOpts controls the unit system for HumanSize.
//
// R1.1: Binary selects between SI (powers of 1000) and binary (powers of 1024) prefixes.
type HumanSizeOpts struct {
	Binary bool // true = 1024-based with Ki/Mi/Gi suffixes; false = 1000-based with K/M/G suffixes
}

// siSuffixes are the unit suffixes for SI (base-1000) mode.
var siSuffixes = []string{"K", "M", "G", "T", "P", "E"}

// binarySuffixes are the unit suffixes for binary (base-1024) mode.
var binarySuffixes = []string{"Ki", "Mi", "Gi", "Ti", "Pi", "Ei"}

// HumanSize converts a byte count to a human-readable string with an appropriate
// unit suffix. It selects the largest unit where the value is >= 1.0 and formats
// the numeric part with one decimal place, dropping the decimal when it is .0.
//
// R1.2: converts byte count to human-readable string with unit suffix.
// R1.3: one decimal place, dropped when .0.
// R1.4: returns exact byte count with no suffix when below the smallest unit threshold.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	// R1.4: zero and values below threshold return the exact count.
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

	value := float64(bytes)

	// R1.4: if value is below the first unit threshold, return exact byte count.
	if value < base {
		return fmt.Sprintf("%d", bytes)
	}

	// R1.3: find the largest unit where value >= 1.0.
	for _, suffix := range suffixes {
		value /= base
		if value < base || suffix == suffixes[len(suffixes)-1] {
			// R1.3: format with one decimal, drop .0 suffix.
			if value == float64(int64(value)) {
				return fmt.Sprintf("%d%s", int64(value), suffix)
			}
			return fmt.Sprintf("%.1f%s", value, suffix)
		}
	}

	// Unreachable: the loop always returns at the last suffix.
	return fmt.Sprintf("%d", bytes)
}
