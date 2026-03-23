// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion and ANSI color output.
//
// Implements prd003-format R3.1–R3.4: HumanSizeOpts, HumanSize.
package format

import (
	"fmt"
)

// HumanSizeOpts controls the unit base for human-readable size formatting.
//
// R3.1: Binary selects 1024-based units (K, M, G, T, P, E) when true,
// and SI 1000-based units (kB, MB, GB, TB) when false.
type HumanSizeOpts struct {
	Binary bool
}

// binarySuffixes are the unit suffixes for 1024-based formatting.
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are the unit suffixes for 1000-based formatting.
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string.
//
// R3.1: uses HumanSizeOpts to select binary (1024) or SI (1000) base.
// R3.2: uses appropriate suffixes for each mode.
// R3.3: formats with one decimal place when the value is fractional.
// R3.4: returns "0" for zero bytes.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	if bytes == 0 {
		return "0"
	}

	base, suffixes := selectUnits(opts)
	value := float64(bytes)

	idx := findUnitIndex(value, base, len(suffixes))
	for range idx {
		value /= base
	}

	return formatValue(value, suffixes[idx])
}

// selectUnits returns the base and suffix list for the given options.
func selectUnits(opts HumanSizeOpts) (float64, []string) {
	if opts.Binary {
		return 1024, binarySuffixes
	}
	return 1000, siSuffixes
}

// findUnitIndex determines which unit suffix index to use.
func findUnitIndex(value, base float64, maxIdx int) int {
	idx := 0
	v := value
	for v >= base && idx < maxIdx-1 {
		v /= base
		idx++
	}
	return idx
}

// formatValue formats the numeric value with its unit suffix.
// Values that are exact integers are formatted without a decimal point
// when the suffix is empty. Fractional values use one decimal place.
func formatValue(value float64, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%d", int64(value))
	}
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d%s", int64(value), suffix)
	}
	return fmt.Sprintf("%.1f%s", value, suffix)
}
