// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "fmt"

// HumanSizeOpts configures the unit mode for HumanSize.
//
// Per prd003-format R3.2.
type HumanSizeOpts struct {
	// SI selects 1000-based units with suffixes kB/MB/GB/TB.
	// When false (default), binary 1024-based units with suffixes K/M/G/T/P/E
	// are used.
	SI bool
}

// binarySuffixes are the unit suffixes for 1024-based conversion.
var binarySuffixes = []string{"K", "M", "G", "T", "P", "E"}

// siSuffixes are the unit suffixes for 1000-based conversion.
var siSuffixes = []string{"kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string with an
// appropriate unit suffix.
//
// Returns "0" for a zero byte count regardless of mode. Formats to at most
// one decimal place when the value is not an integer at the chosen unit
// (e.g., 1536 bytes -> "1.5K").
//
// Per prd003-format R3.1, R3.2, R3.3, R3.4.
// Utility context: ls -h and du -h use this for human-readable output
// (ls.c -h flag, du.c:153, du.c:442).
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	if bytes == 0 {
		return "0"
	}

	var base float64
	var suffixes []string

	if opts.SI {
		base = 1000
		suffixes = siSuffixes
	} else {
		base = 1024
		suffixes = binarySuffixes
	}

	value := float64(bytes)
	for _, suffix := range suffixes {
		next := value / base
		if next < 1.0 {
			// Value does not reach one full unit at this level.
			// At the first level this means sub-K/sub-kB: show raw bytes.
			break
		}
		value = next
		if value < base {
			return formatValue(value, suffix)
		}
	}

	// If we never entered a suffix level (value < base), return raw bytes.
	if value == float64(bytes) {
		return fmt.Sprintf("%d", bytes)
	}

	// Value exceeds all suffix levels; use the largest suffix.
	return formatValue(value, suffixes[len(suffixes)-1])
}

// formatValue formats a floating-point value with one decimal place. If the
// decimal part is zero, it still shows one decimal place (e.g., "1.0K") to
// match GNU coreutils human_readable() behavior.
func formatValue(val float64, suffix string) string {
	return fmt.Sprintf("%.1f%s", val, suffix)
}
