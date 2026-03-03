// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R3: human-readable size conversion.

package format

import "fmt"

// HumanSizeOpts configures the unit system used by HumanSize.
// (prd003-format R3.1)
type HumanSizeOpts struct {
	// Binary selects 1024-based conversion with K/M/G/T/P/E suffixes when
	// true (the zero-value default). When false, selects 1000-based SI
	// conversion with kB/MB/GB/TB suffixes. (prd003-format R3.2)
	Binary bool
}

var (
	binarySuffixes = []string{"", "K", "M", "G", "T", "P", "E"}
	siSuffixes     = []string{"", "kB", "MB", "GB", "TB"}
)

// HumanSize converts bytes to a human-readable string matching GNU coreutils
// human_readable() output for ls -h and du -h. Returns "0" for a zero byte
// count. For values below the first unit boundary the raw integer is returned
// without a suffix. For scaled values, output is formatted to one decimal
// place (e.g., 1536 → "1.5K", 1000000 SI → "1.0MB").
// (prd003-format R3.1–R3.6)
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
	for idx < len(suffixes)-1 && value >= base {
		value /= base
		idx++
	}

	if idx == 0 {
		return fmt.Sprintf("%d", bytes)
	}
	return fmt.Sprintf("%.1f%s", value, suffixes[idx])
}
