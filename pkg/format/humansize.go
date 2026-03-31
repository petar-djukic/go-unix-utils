// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// HumanSize implements prd003 R3.1–R3.4: human-readable size formatting
// matching GNU coreutils human_readable() behavior.

package format

import "fmt"

// HumanSizeOpts configures human-readable size formatting.
//
// R3.1 (prd003): Binary selects 1024-based IEC (Ki, Mi, Gi) vs 1000-based SI (K, M, G) units.
type HumanSizeOpts struct {
	Binary bool
}

// siSuffixes are the 1000-based unit suffixes (prd003 R3.2).
var siSuffixes = []string{"", "K", "M", "G", "T", "P", "E"}

// iecSuffixes are the 1024-based unit suffixes (prd003 R3.2).
var iecSuffixes = []string{"", "Ki", "Mi", "Gi", "Ti", "Pi", "Ei"}

// HumanSize formats a byte count as a human-readable string.
//
// R3.1 (prd003): converts byte count using SI or IEC units depending on opts.Binary.
// R3.3 (prd003): at most one decimal place for non-integer values.
// R3.4 (prd003): returns "0" for zero byte count.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	if bytes == 0 {
		return "0"
	}
	base, suffixes := selectUnits(opts)
	negative := bytes < 0
	val := float64(bytes)
	if negative {
		val = -val
	}
	idx := 0
	for idx < len(suffixes)-1 && val >= base {
		val /= base
		idx++
	}
	result := formatValue(val, suffixes[idx])
	if negative {
		return "-" + result
	}
	return result
}

// selectUnits returns the base and suffix list for the given options.
func selectUnits(opts HumanSizeOpts) (float64, []string) {
	if opts.Binary {
		return 1024.0, iecSuffixes
	}
	return 1000.0, siSuffixes
}

// formatValue renders a scaled value with its suffix using GNU-style precision.
// R3.3 (prd003): one decimal place for values < 100, integer for values >= 100.
func formatValue(val float64, suffix string) string {
	if suffix == "" {
		return fmt.Sprintf("%d", int64(val))
	}
	if val >= 100 {
		return fmt.Sprintf("%d%s", int64(val+0.5), suffix)
	}
	return fmt.Sprintf("%.1f%s", val, suffix)
}
