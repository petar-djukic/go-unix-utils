// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"fmt"
	"math"
)

// HumanSizeOpts configures the unit system for HumanSize (prd003-format R3.1).
type HumanSizeOpts struct {
	// Binary selects 1024-based units with suffixes K/M/G/T/P/E when true,
	// or 1000-based SI units with suffixes kB/MB/GB/TB when false.
	Binary bool
}

// binarySuffixes are the unit suffixes for 1024-based size formatting
// (prd003-format R3.2).
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are the unit suffixes for 1000-based SI size formatting
// (prd003-format R3.2).
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string matching GNU
// coreutils human_readable() behavior (prd003-format R3.1). Values below 10
// at the chosen unit are formatted with one decimal place; values at 10 or
// above use no decimal (prd003-format R3.3). Zero bytes returns "0"
// (prd003-format R3.4).
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

	value := float64(bytes)
	level := 0
	for level < len(suffixes)-1 && math.Abs(value) >= base {
		value /= base
		level++
	}

	if level == 0 {
		return fmt.Sprintf("%d", bytes)
	}

	if math.Abs(value) < 10 {
		return fmt.Sprintf("%.1f%s", value, suffixes[level])
	}
	return fmt.Sprintf("%.0f%s", value, suffixes[level])
}
