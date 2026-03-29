// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

// HumanSizeOpts configures human-readable size formatting.
//
// R1.1 (prd003): Binary selects 1024-based (Ki, Mi, Gi) vs 1000-based (K, M, G) units.
type HumanSizeOpts struct {
	Binary bool
}

// HumanSize formats a byte count as a human-readable string.
//
// R1.2 (prd003): returns SI or IEC suffixed string depending on opts.Binary.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	return ""
}
