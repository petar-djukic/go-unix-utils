// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package sizeparse parses size strings with unit suffixes (K, M, G, etc.)
// into byte counts. Implements srd087-sizeparse R1.1, R1.2, R1.3, R1.4.
package sizeparse

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseOptions configures the behavior of ParseWithOptions.
type ParseOptions struct {
	// AllowSign permits +/- prefix on size strings.
	AllowSign bool
	// DefaultUnit is the multiplier when no suffix is given (default 1).
	DefaultUnit int64
}

// suffixDef defines a unit suffix as a base raised to a power.
type suffixDef struct {
	base  int64
	power int
}

// suffixes maps recognized unit suffixes to their base and power.
// R1.2: binary (IEC) suffixes use base 1024; b uses base 512.
// R1.3: decimal (SI) suffixes use base 1000.
var suffixes = map[string]suffixDef{
	"b":   {512, 1},
	"K":   {1024, 1},
	"KiB": {1024, 1},
	"KB":  {1000, 1},
	"M":   {1024, 2},
	"MiB": {1024, 2},
	"MB":  {1000, 2},
	"G":   {1024, 3},
	"GiB": {1024, 3},
	"GB":  {1000, 3},
	"T":   {1024, 4},
	"TiB": {1024, 4},
	"TB":  {1000, 4},
	"P":   {1024, 5},
	"PiB": {1024, 5},
	"PB":  {1000, 5},
	"E":   {1024, 6},
	"EiB": {1024, 6},
	"EB":  {1000, 6},
	"Z":   {1024, 7},
	"ZiB": {1024, 7},
	"ZB":  {1000, 7},
	"Y":   {1024, 8},
	"YiB": {1024, 8},
	"YB":  {1000, 8},
}

// Parse parses a size string consisting of a decimal integer and an optional
// unit suffix, returning the size in bytes. Returns an error for invalid input.
// R1.1: delegates to ParseWithOptions with default options (D2).
func Parse(s string) (int64, error) {
	return ParseWithOptions(s, ParseOptions{})
}

// ParseWithOptions parses a size string with configurable behavior controlled
// by opts. See ParseOptions for available options.
func ParseWithOptions(s string, opts ParseOptions) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("invalid size: empty string")
	}
	sign, rest, err := extractSign(s, opts.AllowSign)
	if err != nil {
		return 0, err
	}
	num, suffix, err := splitNumSuffix(rest)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}
	multiplier, err := resolveMultiplier(suffix, opts.DefaultUnit)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}
	result, ok := checkedMul(num, multiplier)
	if !ok {
		return 0, fmt.Errorf("size %q overflows int64", s)
	}
	return sign * result, nil
}

// extractSign checks for a leading +/- and returns the sign multiplier,
// the remaining string, and an error if signs are not allowed.
func extractSign(s string, allowSign bool) (int64, string, error) {
	if s[0] != '+' && s[0] != '-' {
		return 1, s, nil
	}
	if !allowSign {
		return 0, "", fmt.Errorf("invalid size %q: sign not allowed", s)
	}
	if s[0] == '-' {
		return -1, s[1:], nil
	}
	return 1, s[1:], nil
}

// splitNumSuffix splits a string into a parsed integer and a suffix string.
func splitNumSuffix(s string) (int64, string, error) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, "", fmt.Errorf("no numeric value")
	}
	num, err := strconv.ParseInt(s[:i], 10, 64)
	if err != nil {
		return 0, "", err
	}
	return num, s[i:], nil
}

// resolveMultiplier returns the byte multiplier for a suffix, or the
// default unit if the suffix is empty.
func resolveMultiplier(suffix string, defaultUnit int64) (int64, error) {
	if suffix == "" {
		if defaultUnit <= 0 {
			return 1, nil
		}
		return defaultUnit, nil
	}
	def, ok := suffixes[suffix]
	if !ok {
		return 0, fmt.Errorf("unrecognized suffix %q", suffix)
	}
	return checkedPow(def.base, def.power)
}

// checkedPow computes base^power with overflow detection.
func checkedPow(base int64, power int) (int64, error) {
	result := int64(1)
	for range power {
		r, ok := checkedMul(result, base)
		if !ok {
			return 0, fmt.Errorf("multiplier overflow")
		}
		result = r
	}
	return result, nil
}

// checkedMul returns a*b and true, or 0 and false on int64 overflow.
func checkedMul(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	result := a * b
	if result/a != b {
		return 0, false
	}
	return result, true
}
