// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, column alignment, and ANSI color output.
//
// Implements prd003-format R3.1-R3.6 (human-readable size conversion).
package format

import (
	"fmt"
	"math"
)

// HumanSizeOpts controls the unit system for human-readable size formatting.
//
// R3.1: Binary selects between 1024-based (true, the default zero value) and
// 1000-based SI (false) unit output.
type HumanSizeOpts struct {
	Binary bool
}

// binarySuffixes are the unit suffixes for 1024-based formatting (R3.2).
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are the unit suffixes for 1000-based SI formatting (R3.2).
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string matching GNU
// coreutils human_readable() behavior (R3.1-R3.6).
//
// When opts.Binary is true (the default zero value), it uses base 1024 with
// suffixes K, M, G, T, P, E. When false, it uses base 1000 with suffixes
// kB, MB, GB, TB.
//
// Output uses at most one decimal place. Zero returns "0".
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	// R3.4: zero returns "0" regardless of mode.
	if bytes == 0 {
		return "0"
	}

	var base float64
	var suffixes []string

	// R3.2: select base and suffixes by mode.
	if opts.Binary {
		base = 1024
		suffixes = binarySuffixes
	} else {
		base = 1000
		suffixes = siSuffixes
	}

	abs := math.Abs(float64(bytes))
	unit := 0
	val := float64(bytes)

	for unit < len(suffixes)-1 && abs >= base {
		val /= base
		abs /= base
		unit++
	}

	// R3.3: at most one decimal place when the value is not an integer.
	if suffixes[unit] == "" {
		// No suffix means raw bytes — format as integer.
		return fmt.Sprintf("%d", bytes)
	}

	// GNU coreutils rounds up: 1.04 → "1.1", not "1.0".
	// Use math.Ceil on (val * 10) / 10 to match ceiling rounding.
	rounded := math.Ceil(math.Abs(val)*10) / 10
	if val < 0 {
		rounded = -rounded
	}

	// If the rounded value is a whole number, format with one decimal.
	// GNU coreutils always shows one decimal for values >= 10 or exact tenths.
	if rounded == math.Trunc(rounded) {
		return fmt.Sprintf("%.1f%s", rounded, suffixes[unit])
	}
	return fmt.Sprintf("%.1f%s", rounded, suffixes[unit])
}
