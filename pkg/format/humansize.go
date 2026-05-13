// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, ANSI color output, and column alignment.
// Implements srd003-format.
package format

// HumanSizeOpts configures human-readable size formatting.
// R1.1: Binary selects 1024-based (Ki, Mi, Gi) vs 1000-based (K, M, G) units.
type HumanSizeOpts struct {
	Binary bool
}

// HumanSize formats a byte count as a human-readable string with unit suffix.
// R1.2: uses SI units by default, IEC binary units when opts.Binary is true.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	panic("not implemented")
}
