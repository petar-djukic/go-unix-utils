// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sizeparse parses human-readable size strings (e.g. "10K", "5M") into
// byte counts. Implements srd087-sizeparse.
package sizeparse

import (
	"fmt"
	"strconv"
)

const maxInt64 = 1<<63 - 1

// ParseOptions configures size string parsing behavior.
type ParseOptions struct {
	AllowSign   bool
	DefaultUnit int64
}

// R1.2: binary (IEC) suffix multipliers, R1.3: decimal (SI) suffix multipliers.
var suffixes = map[string]int64{
	"b":   512,
	"K":   1024,
	"KiB": 1024,
	"KB":  1000,
	"M":   1 << 20,
	"MiB": 1 << 20,
	"MB":  1_000_000,
	"G":   1 << 30,
	"GiB": 1 << 30,
	"GB":  1_000_000_000,
	"T":   1 << 40,
	"TiB": 1 << 40,
	"TB":  1_000_000_000_000,
	"P":   1 << 50,
	"PiB": 1 << 50,
	"PB":  1_000_000_000_000_000,
	"E":   1 << 60,
	"EiB": 1 << 60,
	"EB":  1_000_000_000_000_000_000,
}

// Z/Y suffixes exceed int64 range but must be recognized per R1.2/R1.3.
var overflowSuffixes = map[string]bool{
	"Z": true, "ZiB": true, "ZB": true,
	"Y": true, "YiB": true, "YB": true,
}

// Parse parses a human-readable size string into a byte count.
// R1.1: delegates to ParseWithOptions with default options (no sign, unit=1).
func Parse(s string) (int64, error) {
	return ParseWithOptions(s, ParseOptions{})
}

// ParseWithOptions parses a human-readable size string with configurable options.
// R1.1: accepts optional sign, decimal integer, and optional unit suffix.
func ParseWithOptions(s string, opts ParseOptions) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("invalid size: empty string")
	}
	sign, numStr, suffix, err := splitParts(s, opts.AllowSign)
	if err != nil {
		return 0, err
	}
	n, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}
	multiplier, err := resolveMultiplier(suffix, opts.DefaultUnit)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}
	// R3.1: detect multiplication overflow.
	if multiplier > 1 && n > maxInt64/multiplier {
		return 0, fmt.Errorf("invalid size %q: value overflows int64", s)
	}
	// R1.4: preserve sign in returned value.
	result := n * multiplier
	if sign < 0 {
		result = -result
	}
	return result, nil
}

func splitParts(s string, allowSign bool) (int, string, string, error) {
	sign := 1
	rest := s
	if rest[0] == '+' || rest[0] == '-' {
		if !allowSign {
			return 0, "", "", fmt.Errorf(
				"invalid size %q: sign prefix not allowed", s,
			)
		}
		if rest[0] == '-' {
			sign = -1
		}
		rest = rest[1:]
	}
	i := 0
	for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, "", "", fmt.Errorf("invalid size %q: no numeric value", s)
	}
	return sign, rest[:i], rest[i:], nil
}

func resolveMultiplier(suffix string, defaultUnit int64) (int64, error) {
	if suffix == "" {
		if defaultUnit == 0 {
			return 1, nil
		}
		return defaultUnit, nil
	}
	if overflowSuffixes[suffix] {
		return 0, fmt.Errorf("suffix %q exceeds int64 range", suffix)
	}
	m, ok := suffixes[suffix]
	if !ok {
		return 0, fmt.Errorf("unrecognized suffix %q", suffix)
	}
	return m, nil
}
