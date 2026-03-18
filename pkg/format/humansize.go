// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd003-format R1: human-readable size formatting.
package format

// HumanSizeOpts controls the unit system for HumanSize output.
type HumanSizeOpts struct {
	// Binary selects IEC binary units (Ki, Mi, Gi) when true,
	// and SI decimal units (K, M, G) when false.
	Binary bool
}

// HumanSize formats a byte count as a human-readable string using
// SI or IEC units depending on opts.Binary. Implements prd003-format R1.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	return ""
}
