// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package format provides output formatting shared across utilities including
// human-readable size conversion, ANSI color output, column alignment, and
// string padding.
//
// Implements: srd003-format
package format

import (
	"fmt"
	"io"
	"math"
	"os"
)

// HumanSizeOpts configures human-readable size formatting.
type HumanSizeOpts struct {
	Binary bool
}

// HumanSize formats a byte count as a human-readable string with unit suffixes.
// R3.1: Binary=true uses base 1024 with K/M/G/T/P/E suffixes.
// R3.1: Binary=false uses base 1000 with kB/MB/GB/TB suffixes.
// R3.4: Returns "0" for zero byte count.
func HumanSize(bytes int64, opts HumanSizeOpts) string {
	if bytes == 0 {
		return "0"
	}
	base, suffixes := siParams()
	if opts.Binary {
		base, suffixes = binaryParams()
	}
	return formatHumanSize(bytes, base, suffixes)
}

func binaryParams() (float64, []string) {
	return 1024, []string{"", "K", "M", "G", "T", "P", "E"}
}

func siParams() (float64, []string) {
	return 1000, []string{"", "kB", "MB", "GB", "TB"}
}

// formatHumanSize selects the largest unit where value >= 1.0 and formats
// with GNU coreutils precision: one decimal for values < 10, no decimal otherwise.
// R3.3: At most one decimal place when value is not integer at chosen unit.
func formatHumanSize(bytes int64, base float64, suffixes []string) string {
	neg := bytes < 0
	val := math.Abs(float64(bytes))
	idx := 0
	for idx+1 < len(suffixes) && val >= base {
		val /= base
		idx++
	}
	prefix := ""
	if neg {
		prefix = "-"
	}
	if suffixes[idx] == "" {
		return fmt.Sprintf("%s%d", prefix, abs64(bytes))
	}
	// R3.4/R3.3: GNU format — one decimal when < 10, no decimal when >= 10.
	if val < 10.0 {
		return fmt.Sprintf("%s%.1f%s", prefix, val, suffixes[idx])
	}
	return fmt.Sprintf("%s%.0f%s", prefix, val, suffixes[idx])
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// FileTypeColor returns the ANSI color escape sequence for the given file mode.
func FileTypeColor(mode os.FileMode) string {
	return ""
}

// Reset returns the ANSI reset escape sequence.
func Reset() string {
	return ""
}

// ColorEnabled reports whether color output is enabled for the given writer.
func ColorEnabled(w io.Writer) bool {
	return false
}

// SetColorEnabled overrides the automatic color detection with the given value.
func SetColorEnabled(enabled bool) {
}

// ResetColorEnabled clears any manual color override, restoring automatic detection.
func ResetColorEnabled() {
}

// PadRight pads s on the right with spaces to the given width.
func PadRight(s string, width int) string {
	return ""
}

// PadLeft pads s on the left with spaces to the given width.
func PadLeft(s string, width int) string {
	return ""
}

// Columns arranges entries into columns that fit within termWidth.
func Columns(entries []string, termWidth int) [][]string {
	return nil
}
