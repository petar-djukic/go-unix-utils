// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable unit conversion, table alignment, and ANSI color output.
//
// Implements: prd003-format R3.1–R3.6 (HumanSize, HumanSizeOpts).
package format

import (
	"fmt"
	"math"
)

// HumanSizeOpts controls the unit system used by HumanSize. R3.1.
type HumanSizeOpts struct {
	// Binary selects 1024-based units with suffixes K/M/G/T/P/E when true,
	// and 1000-based SI units with suffixes kB/MB/GB/TB when false. R3.2.
	Binary bool
}

// binarySuffixes are the IEC-style suffixes used by GNU coreutils -h flags.
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are the SI-style suffixes used by GNU coreutils --si flags.
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string matching GNU
// coreutils human_readable() output format. R3.1–R3.6.
//
// When opts.Binary is true, uses base 1024 with suffixes K, M, G, T, P, E.
// When opts.Binary is false, uses base 1000 with suffixes kB, MB, GB, TB.
//
// Formatting rules (R3.3, R3.4):
//   - Zero returns "0".
//   - Values under 10 at a given unit show one decimal digit (e.g., "1.5K").
//   - Values 10 and above at a given unit show no decimal (e.g., "15K").
//   - No trailing zeros beyond the required format.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	// R3.4: zero is always "0".
	if bytes == 0 {
		return "0"
	}

	negative := bytes < 0
	val := math.Abs(float64(bytes))

	var base float64
	var suffixes []string
	if opts.Binary {
		base = 1024
		suffixes = binarySuffixes
	} else {
		base = 1000
		suffixes = siSuffixes
	}

	// Find the appropriate unit.
	idx := 0
	for idx < len(suffixes)-1 && val >= base {
		val /= base
		idx++
	}

	prefix := ""
	if negative {
		prefix = "-"
	}

	// R3.3, R3.4: format with at most one decimal place.
	if idx == 0 {
		// Still in bytes, no suffix — show as integer.
		return fmt.Sprintf("%s%d", prefix, int64(math.Round(val)))
	}

	// R3.3: one decimal digit for values under 10 at the chosen unit.
	// Check the rounded-to-one-decimal value to avoid "10.0K" for 9.999K.
	rounded := math.Round(val*10) / 10
	if rounded < 10 {
		return fmt.Sprintf("%s%.1f%s", prefix, val, suffixes[idx])
	}

	// No decimal for values 10 and above.
	return fmt.Sprintf("%s%d%s", prefix, int64(math.Round(val)), suffixes[idx])
}
