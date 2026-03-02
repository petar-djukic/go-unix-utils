// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, column alignment, and ANSI color output.
//
// Implements: prd003-format R1, R2, R3
package format

import (
	"fmt"
	"math"
)

// HumanSizeOpts configures the unit system for HumanSize.
type HumanSizeOpts struct {
	// Binary selects 1024-based units with suffixes K/M/G/T/P/E (default).
	// When false, uses 1000-based SI units with suffixes kB/MB/GB/TB.
	Binary bool
}

var (
	binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}
	siSuffixes     = []string{"", "kB", "MB", "GB", "TB"}
)

// HumanSize converts a byte count to a human-readable string matching GNU
// coreutils human_readable() behavior.
//
// Implements: prd003-format R3.1, R3.2, R3.3, R3.4
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
	for i, suffix := range suffixes {
		next := value / base
		if next < 1 || i == len(suffixes)-1 {
			return formatValue(value, suffix)
		}
		value = next
	}

	// Unreachable: the loop always returns.
	last := suffixes[len(suffixes)-1]
	return formatValue(value, last)
}

// formatValue formats a numeric value with its unit suffix following GNU
// coreutils conventions: no suffix (bytes) uses integer format; values < 10
// at a scaled unit show one decimal place; values >= 10 show integer format.
func formatValue(value float64, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%d", int64(value))
	}
	if value < 10 {
		return fmt.Sprintf("%.1f%s", value, suffix)
	}
	return fmt.Sprintf("%d%s", int64(math.Round(value)), suffix)
}
