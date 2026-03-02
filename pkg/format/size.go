// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across go-unix-utils utilities
// including column alignment, ANSI color output, and human-readable size conversion.
//
// Implements: prd003-format (R3)
package format

import "fmt"

// HumanSizeOpts configures the behavior of HumanSize.
// (prd003-format R3.1)
type HumanSizeOpts struct {
	Binary bool // true = 1024-based with suffixes K/M/G/T/P/E; false = SI 1000-based with suffixes kB/MB/GB/TB
}

// binarySuffixes are the unit suffixes for 1024-based human-readable sizes.
// (prd003-format R3.2)
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are the unit suffixes for 1000-based (SI) human-readable sizes.
// (prd003-format R3.2)
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string. When opts.Binary
// is true, it uses base 1024 with suffixes K/M/G/T/P/E. When opts.Binary is
// false, it uses base 1000 with suffixes kB/MB/GB/TB. Output uses at most one
// decimal place. Zero bytes returns "0" regardless of mode.
// (prd003-format R3.1, R3.2, R3.3, R3.4, R3.5, R3.6)
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

	value := float64(bytes)
	for i, suffix := range suffixes {
		if i == len(suffixes)-1 || value < base {
			if suffix == "" {
				return fmt.Sprintf("%d", int64(value))
			}
			return fmt.Sprintf("%.1f%s", value, suffix)
		}
		value /= base
	}

	// Unreachable: the loop always returns via the last-suffix or under-base check.
	return "0"
}
