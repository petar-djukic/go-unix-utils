// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities.
// Implements prd003-format R1.1-R1.4: human-readable size formatting.
// R3.5: ls -h uses HumanSize for file sizes in -l output and block counts in -s output.
// R3.6: du -h uses HumanSize for directory sizes with the same binary/SI distinction.
package format

import (
	"fmt"
	"math"
	"strings"
)

// HumanSizeOpts controls the behavior of HumanSize.
// R1.1: Binary selects IEC base-1024 (Ki, Mi, Gi) when true,
// or SI base-1000 (K, M, G) when false.
type HumanSizeOpts struct {
	Binary bool
}

// siSuffixes are the SI unit suffixes (base 1000).
var siSuffixes = []string{"B", "K", "M", "G", "T", "P", "E"}

// iecSuffixes are the IEC/binary unit suffixes (base 1024).
var iecSuffixes = []string{"B", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei"}

// HumanSize converts a byte count to a human-readable string with an
// appropriate unit suffix. It selects the largest unit that keeps the
// numeric value >= 1.0, formatting with one decimal place and no
// trailing zeros.
//
// R1.2: Returns a string with the appropriate suffix based on opts.Binary.
// R1.3: Selects largest unit keeping value >= 1.0, one decimal place, no trailing zeros.
// R1.4: Handles zero (returns "0B"), negative values (leading '-'), and exact boundaries.
// R3.5: Used by ls -h for file sizes and block counts.
// R3.6: Used by du -h for directory sizes with the same binary/SI distinction.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	if bytes == 0 {
		return "0B"
	}

	negative := bytes < 0
	abs := float64(bytes)
	if negative {
		abs = -abs
	}

	var base float64
	var suffixes []string
	if opts.Binary {
		base = 1024
		suffixes = iecSuffixes
	} else {
		base = 1000
		suffixes = siSuffixes
	}

	// Find the largest unit where value >= 1.0
	val := abs
	idx := 0
	for idx < len(suffixes)-1 && val >= base {
		val /= base
		idx++
	}

	// Format with one decimal place, then trim trailing zero and dot
	var formatted string
	if idx == 0 {
		// Bytes: always integer
		formatted = fmt.Sprintf("%d", int64(math.Round(val)))
	} else {
		formatted = fmt.Sprintf("%.1f", val)
		formatted = strings.TrimRight(formatted, "0")
		formatted = strings.TrimRight(formatted, ".")
	}

	if negative {
		return "-" + formatted + suffixes[idx]
	}
	return formatted + suffixes[idx]
}
