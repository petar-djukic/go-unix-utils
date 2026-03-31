// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// parse.go implements prd087 R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R3.1, R3.2:
// core size string parsing with SI and IEC unit suffix support,
// configurable sign handling, default unit, and error reporting.

package sizeparse

import (
	"fmt"
	"strconv"
	"strings"
)

// suffixMultipliers maps recognized unit suffixes (uppercase) to byte multipliers.
// R1.2: binary/IEC suffixes (powers of 1024).
// R1.3: decimal/SI suffixes (powers of 1000).
// Note: Z/Y suffixes overflow int64 and are handled as runtime errors in
// multiplyChecked rather than being excluded from the map entirely.
var suffixMultipliers = map[string]int64{
	"B": 1,

	// R1.2: POSIX block suffix (handled specially for case sensitivity).
	"b": 512,

	// R1.2: binary suffixes (powers of 1024).
	"K":   1024,
	"KIB": 1024,
	"M":   1024 * 1024,
	"MIB": 1024 * 1024,
	"G":   1024 * 1024 * 1024,
	"GIB": 1024 * 1024 * 1024,
	"T":   1024 * 1024 * 1024 * 1024,
	"TIB": 1024 * 1024 * 1024 * 1024,
	"P":   1024 * 1024 * 1024 * 1024 * 1024,
	"PIB": 1024 * 1024 * 1024 * 1024 * 1024,
	"E":   1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	"EIB": 1024 * 1024 * 1024 * 1024 * 1024 * 1024,

	// R1.3: decimal suffixes (powers of 1000).
	"KB": 1000,
	"MB": 1000 * 1000,
	"GB": 1000 * 1000 * 1000,
	"TB": 1000 * 1000 * 1000 * 1000,
	"PB": 1000 * 1000 * 1000 * 1000 * 1000,
	"EB": 1000 * 1000 * 1000 * 1000 * 1000 * 1000,
}

// parseSizeString parses a size string into its numeric value in bytes.
// R1.1: parses decimal integer with optional unit suffix.
// R1.4: preserves sign in returned value.
func parseSizeString(s string, opts ParseOptions) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid size: empty string")
	}
	num, suffix, err := splitNumSuffix(s, opts.AllowSign)
	if err != nil {
		return 0, err
	}
	multiplier, err := lookupMultiplier(suffix, opts.DefaultUnit)
	if err != nil {
		return 0, err
	}
	return multiplyChecked(num, multiplier)
}

// splitNumSuffix separates the numeric part from the suffix.
// R1.4: handles optional +/- sign prefix.
func splitNumSuffix(s string, allowSign bool) (int64, string, error) {
	start := 0
	sign := int64(1)
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		if !allowSign {
			return 0, "", fmt.Errorf("invalid size %q: sign not allowed", s)
		}
		if s[0] == '-' {
			sign = -1
		}
		start = 1
	}
	end := findSuffixStart(s, start)
	numStr := s[start:end]
	if numStr == "" {
		return 0, "", fmt.Errorf("invalid size %q: no numeric value", s)
	}
	num, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("invalid size %q: %w", s, err)
	}
	return num * sign, s[end:], nil
}

// findSuffixStart returns the index where the suffix begins (first non-digit
// after the numeric part).
func findSuffixStart(s string, start int) int {
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return i
		}
	}
	return len(s)
}

// lookupMultiplier returns the byte multiplier for the given suffix.
// R1.2, R1.3: case-insensitive lookup except for 'b' (512) vs 'B' (1).
// R3.2: returns error for unrecognized suffixes.
func lookupMultiplier(suffix string, defaultUnit int64) (int64, error) {
	if suffix == "" {
		if defaultUnit == 0 {
			return 1, nil
		}
		return defaultUnit, nil
	}
	// Special case: lowercase 'b' means 512 (POSIX blocks).
	if suffix == "b" {
		return 512, nil
	}
	upper := strings.ToUpper(suffix)
	m, ok := suffixMultipliers[upper]
	if !ok {
		return 0, fmt.Errorf("invalid suffix %q", suffix)
	}
	return m, nil
}

// multiplyChecked multiplies num by multiplier with int64 overflow detection.
// R3.1: returns error on overflow.
func multiplyChecked(num, multiplier int64) (int64, error) {
	if multiplier == 1 {
		return num, nil
	}
	if num == 0 {
		return 0, nil
	}
	result := num * multiplier
	if result/multiplier != num {
		return 0, fmt.Errorf("size overflow: %d * %d exceeds int64", num, multiplier)
	}
	return result, nil
}
