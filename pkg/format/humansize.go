// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

// HumanSizeOpts configures human-readable size formatting.
type HumanSizeOpts struct {
	Binary bool
}

// HumanSize formats a byte count as a human-readable string with unit suffix.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	panic("not implemented")
}
