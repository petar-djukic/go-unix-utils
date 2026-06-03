// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "fmt"

// HumanSizeOpts configures human-readable size formatting.
type HumanSizeOpts struct {
	Binary bool
}

// HumanSize formats a byte count as a human-readable string with unit suffix.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	if bytes == 0 {
		return "0"
	}

	var base float64
	var suffixes []string

	if opts.Binary {
		base = 1024
		suffixes = []string{"", "K", "M", "G", "T", "P", "E"}
	} else {
		base = 1000
		suffixes = []string{"", "kB", "MB", "GB", "TB"}
	}

	val := float64(bytes)
	for i := 0; i < len(suffixes)-1; i++ {
		if val < base {
			if suffixes[i] == "" {
				return fmt.Sprintf("%d", int64(val))
			}
			if val < 10 {
				return fmt.Sprintf("%.1f%s", val, suffixes[i])
			}
			return fmt.Sprintf("%d%s", int64(val), suffixes[i])
		}
		val /= base
	}

	last := suffixes[len(suffixes)-1]
	if val < 10 {
		return fmt.Sprintf("%.1f%s", val, last)
	}
	return fmt.Sprintf("%d%s", int64(val), last)
}
