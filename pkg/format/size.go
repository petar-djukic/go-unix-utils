// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "fmt"

// HumanSizeOpts configures the unit convention used by HumanSize.
// Implements prd003-format R3.1.
type HumanSizeOpts struct {
	// Binary selects 1024-based conversion with K/M/G/T/P/E suffixes when
	// true. When false, SI 1000-based conversion with kB/MB/GB/TB suffixes
	// is used.
	Binary bool
}

// binarySuffixes lists the unit suffixes for 1024-based conversion.
var binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"} //nolint:gochecknoglobals

// siSuffixes lists the unit suffixes for 1000-based (SI) conversion.
var siSuffixes = []string{"", "kB", "MB", "GB", "TB"} //nolint:gochecknoglobals

// HumanSize converts bytes to a human-readable string using the chosen unit
// convention. A zero byte count always returns "0". Values below the first
// unit threshold are returned as a plain decimal integer with no suffix.
// Larger values are formatted with exactly one decimal place and the
// appropriate suffix, e.g., "1.5K" or "2.0MB", matching GNU coreutils
// human_readable() output.
//
// Implements prd003-format R3.1–R3.6.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	// R3.4: zero always returns "0" regardless of unit mode.
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
	for idx < len(suffixes)-1 && value >= base {
		value /= base
		idx++
	}

	if idx == 0 {
		// Value is below the first threshold: return a plain integer.
		return fmt.Sprintf("%d", bytes)
	}
	// R3.3: one decimal place for all suffixed values.
	return fmt.Sprintf("%.1f%s", value, suffixes[idx])
}
