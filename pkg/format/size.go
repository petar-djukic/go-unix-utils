// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, ANSI color output, and column alignment.
//
// Implements prd003-format (R1-R3).
package format

import (
	"fmt"
	"math"
)

// HumanSizeOpts configures the unit system for human-readable size conversion.
// R3.1: Binary selects between 1024-based and 1000-based unit systems.
type HumanSizeOpts struct {
	// Binary selects 1024-based units with suffixes K/M/G/T/P/E when true
	// (the default zero value). When false, selects 1000-based SI units with
	// suffixes kB/MB/GB/TB.
	Binary bool
}

// binarySuffixes are the unit suffixes for 1024-based conversion.
// R3.2: base-1024 suffixes.
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are the unit suffixes for 1000-based SI conversion.
// R3.2: base-1000 suffixes.
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"}

// HumanSize converts a byte count to a human-readable string using the unit
// system specified by opts. Values less than 10 at the chosen unit are formatted
// with one decimal place; values 10 or greater use no decimal place.
// R3.1-R3.4: human-readable size conversion matching GNU coreutils behavior.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	// R3.4: zero returns "0" regardless of unit mode.
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

	size := float64(bytes)
	for i := 0; i < len(suffixes); i++ {
		if i == len(suffixes)-1 || math.Abs(size) < base {
			if suffixes[i] == "" {
				return fmt.Sprintf("%d", bytes)
			}
			// R3.3: at most one decimal place when value is not integer.
			if math.Abs(size) < 10 {
				return fmt.Sprintf("%.1f%s", size, suffixes[i])
			}
			return fmt.Sprintf("%.0f%s", size, suffixes[i])
		}
		size /= base
	}

	return fmt.Sprintf("%d", bytes)
}
