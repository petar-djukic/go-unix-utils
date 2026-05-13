// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, ANSI color output, and column alignment.
// Implements srd003-format.
package format

import "fmt"

// HumanSizeOpts configures human-readable size formatting.
// R3.1: Binary selects 1024-based (K, M, G) vs 1000-based (kB, MB, GB) units.
type HumanSizeOpts struct {
	Binary bool
}

var (
	binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}
	siSuffixes     = []string{"", "kB", "MB", "GB", "TB"}
)

// HumanSize formats a byte count as a human-readable string with unit suffix.
// R3.2: binary mode uses base 1024; SI mode uses base 1000.
// R3.3: outputs at most one decimal place when the value is fractional.
// R3.4: returns "0" for zero bytes.
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

	val := float64(bytes)
	for i, suffix := range suffixes {
		next := val / base
		if next < 1 || i == len(suffixes)-1 {
			return formatValue(val, suffix)
		}
		val = next
	}
	return formatValue(val, suffixes[len(suffixes)-1])
}

func formatValue(val float64, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%d", int64(val))
	}
	return fmt.Sprintf("%.1f%s", val, suffix)
}
