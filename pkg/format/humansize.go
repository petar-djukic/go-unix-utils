// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, ANSI color output, and column alignment.
// Implements prd003-format R1-R4.
package format

import "fmt"

// HumanSizeOpts controls the unit system for HumanSize.
type HumanSizeOpts struct {
	Binary bool // true = 1024-based (K, M, G); false = 1000-based (kB, MB, GB)
}

// binarySuffixes are the unit suffixes for 1024-based human-readable output.
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// siSuffixes are the unit suffixes for 1000-based human-readable output.
var siSuffixes = []string{"", "kB", "MB", "GB", "TB", "PB", "EB"}

// HumanSize converts a byte count to a human-readable string.
// R3.1-R3.4: matches GNU coreutils human_readable() output format.
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
	idx := 0
	for value >= base && idx < len(suffixes)-1 {
		value /= base
		idx++
	}

	if idx == 0 {
		return fmt.Sprintf("%d", bytes)
	}

	// R3.3: one decimal place when not an integer, omit decimal when integer.
	// Binary mode omits decimal for integer values; SI mode always shows one decimal
	// to match GNU coreutils numfmt --to=si output.
	if opts.Binary && value == float64(int64(value)) {
		return fmt.Sprintf("%d%s", int64(value), suffixes[idx])
	}
	return fmt.Sprintf("%.1f%s", value, suffixes[idx])
}
