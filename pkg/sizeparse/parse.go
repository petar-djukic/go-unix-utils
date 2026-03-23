// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sizeparse provides size string parsing with unit suffixes for
// cmd/ utilities that accept human-readable size arguments.
//
// Implements prd087-sizeparse R1.1–R1.4, R2.1–R2.2, R3.1–R3.2.
package sizeparse

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ParseOptions configures behavior for ParseWithOptions.
//
// R2.1: AllowSign controls whether +/- prefix is accepted.
// DefaultUnit is the multiplier when no suffix is given (default 1).
type ParseOptions struct {
	AllowSign   bool
	DefaultUnit int64
}

// suffixMultiplier maps recognized suffixes to their byte multipliers.
// R1.2: binary (IEC) suffixes; R1.3: decimal (SI) suffixes.
// Z and Y suffixes overflow int64 and are handled in overflowSuffix.
var suffixMultiplier = map[string]int64{
	"b":   512,
	"K":   1024,
	"KiB": 1024,
	"KB":  1000,
	"M":   1024 * 1024,
	"MiB": 1024 * 1024,
	"MB":  1000 * 1000,
	"G":   1024 * 1024 * 1024,
	"GiB": 1024 * 1024 * 1024,
	"GB":  1000 * 1000 * 1000,
	"T":   1024 * 1024 * 1024 * 1024,
	"TiB": 1024 * 1024 * 1024 * 1024,
	"TB":  1000 * 1000 * 1000 * 1000,
	"P":   1024 * 1024 * 1024 * 1024 * 1024,
	"PiB": 1024 * 1024 * 1024 * 1024 * 1024,
	"PB":  1000 * 1000 * 1000 * 1000 * 1000,
	"E":   1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	"EiB": 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	"EB":  1000 * 1000 * 1000 * 1000 * 1000 * 1000,
}

// overflowSuffix lists suffixes whose multipliers exceed int64.
// R1.2/R1.3: Z and Y are valid suffixes but always overflow for n > 0.
var overflowSuffix = map[string]bool{
	"Z": true, "ZiB": true, "ZB": true,
	"Y": true, "YiB": true, "YB": true,
}

// Parse parses a size string consisting of a decimal integer and an
// optional unit suffix, returning the size in bytes.
//
// R1.1: delegates to ParseWithOptions with default options.
func Parse(s string) (int64, error) {
	return ParseWithOptions(s, ParseOptions{})
}

// ParseWithOptions parses a size string with configurable behavior
// controlled by opts.
//
// R2.1: AllowSign enables +/- prefix. R2.2: DefaultUnit applies when
// no suffix is present.
func ParseWithOptions(s string, opts ParseOptions) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid size: %q", s)
	}
	sign, rest := extractSign(s, opts.AllowSign)
	if rest == "" {
		return 0, fmt.Errorf("invalid size: %q", s)
	}
	numStr, suffix := splitNumSuffix(rest)
	if numStr == "" {
		return 0, fmt.Errorf("invalid size: %q", s)
	}
	return computeResult(s, numStr, suffix, sign, opts)
}

// extractSign strips a leading +/- when allowed and returns the sign
// multiplier (1 or -1) and the remaining string.
func extractSign(s string, allow bool) (int64, string) {
	if !allow || len(s) == 0 {
		return 1, s
	}
	switch s[0] {
	case '+':
		return 1, s[1:]
	case '-':
		return -1, s[1:]
	default:
		return 1, s
	}
}

// splitNumSuffix splits a string into the leading digit sequence and
// the trailing suffix.
func splitNumSuffix(s string) (string, string) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i], s[i:]
}

// computeResult parses the numeric part, resolves the suffix, and
// returns the final byte count with overflow checking.
// R1.4: sign is applied. R3.1: overflow detected. R3.2: unknown suffix.
func computeResult(
	orig, numStr, suffix string, sign int64, opts ParseOptions,
) (int64, error) {
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size: %q", orig)
	}
	mult, err := resolveMultiplier(orig, suffix, n, opts.DefaultUnit)
	if err != nil {
		return 0, err
	}
	return multiplyChecked(orig, n, mult, sign)
}

// resolveMultiplier returns the byte multiplier for a given suffix.
// When suffix is empty, DefaultUnit is used (falling back to 1).
func resolveMultiplier(
	orig, suffix string, n, defaultUnit int64,
) (int64, error) {
	if suffix == "" {
		if defaultUnit > 0 {
			return defaultUnit, nil
		}
		return 1, nil
	}
	return lookupSuffix(orig, suffix, n)
}

// lookupSuffix resolves a non-empty suffix to its multiplier.
// R3.2: returns error for unknown suffixes.
func lookupSuffix(orig, suffix string, n int64) (int64, error) {
	if overflowSuffix[suffix] || overflowSuffix[strings.ToUpper(suffix)] {
		if n == 0 {
			return 1, nil
		}
		return 0, fmt.Errorf("size overflow: %q", orig)
	}
	m, ok := suffixMultiplier[suffix]
	if !ok {
		// R1.2: accept lowercase single-letter as binary suffix.
		m, ok = suffixMultiplier[strings.ToUpper(suffix)]
	}
	if !ok {
		return 0, fmt.Errorf("invalid suffix in size: %q", orig)
	}
	return m, nil
}

// multiplyChecked multiplies n * mult * sign with int64 overflow
// detection.
// R3.1: returns an error when the result overflows int64.
func multiplyChecked(
	orig string, n, mult, sign int64,
) (int64, error) {
	if mult != 0 && n > math.MaxInt64/mult {
		return 0, fmt.Errorf("size overflow: %q", orig)
	}
	result := n * mult
	if sign < 0 {
		result = -result
	}
	return result, nil
}
