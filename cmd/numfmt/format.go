// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd071-numfmt R2.1 (--format), R2.2 (--padding): format spec
// parsing, formatted output, and padding logic.
package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// formatSpec holds parsed --format directive components. R2.1.
type formatSpec struct {
	prefix    string
	leftAlign bool
	width     int
	precision int // -1 = not specified
	trailing  string
}

// parseFormatStr parses a --format string into a formatSpec.
// The format must contain exactly one %f directive. R2.1.
func parseFormatStr(format string) (formatSpec, error) {
	pctIdx := -1
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		if i+1 < len(format) && format[i+1] == '%' {
			i++ // skip %%
			continue
		}
		pctIdx = i
		break
	}
	if pctIdx < 0 {
		return formatSpec{}, fmt.Errorf("format '%s' has no %% directive", format)
	}
	return parseDirective(format, pctIdx)
}

// parseDirective extracts formatSpec from format string at pctIdx.
func parseDirective(format string, pctIdx int) (formatSpec, error) {
	spec := formatSpec{prefix: format[:pctIdx], precision: -1}
	rest := format[pctIdx+1:]
	pos := 0
	if pos < len(rest) && rest[pos] == '-' {
		spec.leftAlign = true
		pos++
	}
	// Parse width.
	start := pos
	for pos < len(rest) && rest[pos] >= '0' && rest[pos] <= '9' {
		pos++
	}
	if pos > start {
		spec.width, _ = strconv.Atoi(rest[start:pos])
	}
	// Parse precision.
	if pos < len(rest) && rest[pos] == '.' {
		pos++
		start = pos
		for pos < len(rest) && rest[pos] >= '0' && rest[pos] <= '9' {
			pos++
		}
		spec.precision = 0
		if pos > start {
			spec.precision, _ = strconv.Atoi(rest[start:pos])
		}
	}
	if pos >= len(rest) || rest[pos] != 'f' {
		return formatSpec{}, fmt.Errorf("format '%s' has no %%f directive", format)
	}
	spec.trailing = rest[pos+1:]
	return spec, nil
}

// formatWithSpec formats using the parsed --format specification. R2.1.
func formatWithSpec(val float64, cfg numfmtConfig) string {
	spec := cfg.fmtSpec
	numStr := formatNumForSpec(val, cfg.to, cfg.round, spec.precision)
	numStr += cfg.suffix
	padded := padToWidth(numStr, spec.width, spec.leftAlign)
	return spec.prefix + padded + spec.trailing
}

// formatNumForSpec formats a number for --format output.
func formatNumForSpec(val float64, unit scaleUnit, mode roundMode, prec int) string {
	if unit == scaleNone {
		return formatWithPrecision(val, "", prec, mode)
	}
	base := baseForUnit(unit)
	idx := findBestScale(math.Abs(val), base)
	if idx < 0 {
		return formatWithPrecision(val, "", prec, mode)
	}
	scaled := val / math.Pow(base, float64(idx+1))
	return formatWithPrecision(scaled, buildSuffix(idx, unit), prec, mode)
}

// formatWithPrecision formats a number with optional explicit precision.
func formatWithPrecision(val float64, suffix string, prec int, mode roundMode) string {
	if prec < 0 {
		if suffix == "" {
			return formatRaw(val)
		}
		return formatScaled(val, suffix, mode)
	}
	rounded := roundValue(val, prec, mode)
	if suffix == "" {
		return fmt.Sprintf("%.*f", prec, rounded)
	}
	return fmt.Sprintf("%.*f%s", prec, rounded, suffix)
}

// applyPadding pads the result to the specified width. R2.2.
func applyPadding(s string, padding int) string {
	if padding == 0 {
		return s
	}
	w := padding
	if w < 0 {
		w = -w
	}
	return padToWidth(s, w, padding < 0)
}

// padToWidth pads s to the given width with spaces.
func padToWidth(s string, width int, leftAlign bool) string {
	if width <= len(s) {
		return s
	}
	pad := strings.Repeat(" ", width-len(s))
	if leftAlign {
		return s + pad
	}
	return pad + s
}
