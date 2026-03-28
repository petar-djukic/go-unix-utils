// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Comparison functions for sort modes: general numeric (-g),
// human-readable numeric (-h), version sort (-V), fold case (-f),
// dictionary order (-d), and ignore non-printing (-i).
// Implements prd053-sort R3.1, R3.2.
package main

import (
	"bytes"
	"math"
	"strconv"
	"strings"
)

// compareFloats compares two float64 values, treating NaN as less than any number.
// D1: NaN sorts before all other values for -g mode.
func compareFloats(a, b float64) int {
	aNaN := math.IsNaN(a)
	bNaN := math.IsNaN(b)
	if aNaN && bNaN {
		return 0
	}
	if aNaN {
		return -1
	}
	if bNaN {
		return 1
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// generalNumericCompare compares by general floating-point value.
// R3.1: -g parses with strconv.ParseFloat including scientific notation.
func generalNumericCompare(a, b []byte) int {
	va := parseGeneralFloat(a)
	vb := parseGeneralFloat(b)
	return compareFloats(va, vb)
}

// parseGeneralFloat parses a trimmed byte slice as a float64.
// Returns NaN for unparseable values.
func parseGeneralFloat(b []byte) float64 {
	s := strings.TrimSpace(string(b))
	if len(s) == 0 {
		return math.NaN()
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return math.NaN()
	}
	return val
}

// humanNumericCompare compares by numeric value with SI suffixes.
// D2: K=1024, M=1024^2, G=1024^3, etc.
func humanNumericCompare(a, b []byte) int {
	va := parseHumanNumber(a)
	vb := parseHumanNumber(b)
	return compareFloats(va, vb)
}

// parseHumanNumber extracts a number with optional SI suffix.
func parseHumanNumber(b []byte) float64 {
	s := strings.TrimSpace(string(b))
	if len(s) == 0 {
		return 0
	}
	sign, rest := parseHumanSign(s)
	num, suffix := splitHumanParts(rest)
	if num == "" {
		return 0
	}
	val, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	return sign * val * humanSuffixMultiplier(suffix)
}

// parseHumanSign extracts sign prefix and returns multiplier and rest.
func parseHumanSign(s string) (float64, string) {
	if len(s) > 0 && s[0] == '-' {
		return -1.0, s[1:]
	}
	if len(s) > 0 && s[0] == '+' {
		return 1.0, s[1:]
	}
	return 1.0, s
}

// splitHumanParts splits a string into numeric and suffix portions.
func splitHumanParts(s string) (string, string) {
	i := 0
	for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == '.') {
		i++
	}
	return s[:i], s[i:]
}

// humanSuffixMultiplier returns the multiplier for an SI suffix.
func humanSuffixMultiplier(suffix string) float64 {
	if len(suffix) == 0 {
		return 1
	}
	const ki = 1024
	switch suffix[0] {
	case 'K':
		return ki
	case 'M':
		return ki * ki
	case 'G':
		return ki * ki * ki
	case 'T':
		return ki * ki * ki * ki
	case 'P':
		return ki * ki * ki * ki * ki
	case 'E':
		return ki * ki * ki * ki * ki * ki
	case 'Z':
		return ki * ki * ki * ki * ki * ki * ki
	case 'Y':
		return ki * ki * ki * ki * ki * ki * ki * ki
	default:
		return 1
	}
}

// versionCompare compares by version number segments.
// D3: split into numeric and non-numeric runs, compare naturally.
func versionCompare(a, b []byte) int {
	return compareVersionStrings(string(a), string(b))
}

// compareVersionStrings splits two strings into version segments and compares.
func compareVersionStrings(a, b string) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		ca, cb := a[ai], b[bi]
		if isDigit(ca) && isDigit(cb) {
			cmp, na, nb := compareNumericRun(a, ai, b, bi)
			if cmp != 0 {
				return cmp
			}
			ai, bi = na, nb
			continue
		}
		if ca != cb {
			return byteOrder(ca, cb)
		}
		ai++
		bi++
	}
	return lengthOrder(len(a)-ai, len(b)-bi)
}

// compareNumericRun compares numeric runs starting at given positions.
func compareNumericRun(a string, ai int, b string, bi int) (int, int, int) {
	na := scanDigitRun(a, ai)
	nb := scanDigitRun(b, bi)
	cmp := compareNumericStrings(a[ai:na], b[bi:nb])
	return cmp, na, nb
}

// scanDigitRun returns the end index of a digit run.
func scanDigitRun(s string, start int) int {
	i := start
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

// compareNumericStrings compares two numeric strings by value.
func compareNumericStrings(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if len(a) != len(b) {
		return lengthOrder(len(a), len(b))
	}
	return strings.Compare(a, b)
}

// isDigit returns true if b is an ASCII digit.
func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

// byteOrder returns comparison result for two bytes.
func byteOrder(a, b byte) int {
	if a < b {
		return -1
	}
	return 1
}

// lengthOrder returns comparison result for two lengths.
func lengthOrder(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// transformForCompare applies text transforms before lexicographic comparison.
func transformForCompare(b []byte, mods keyMods) []byte {
	result := b
	if mods.ignoreNP {
		result = filterPrintable(result)
	}
	if mods.dict {
		result = filterDict(result)
	}
	if mods.foldCase {
		result = foldToUpper(result)
	}
	return result
}

// filterPrintable removes non-printable characters (outside 32-126).
func filterPrintable(b []byte) []byte {
	var buf bytes.Buffer
	for _, c := range b {
		if c >= 32 && c <= 126 {
			buf.WriteByte(c)
		}
	}
	return buf.Bytes()
}

// filterDict keeps only blanks and alphanumeric characters.
func filterDict(b []byte) []byte {
	var buf bytes.Buffer
	for _, c := range b {
		if isBlank(c) || isAlphaNum(c) {
			buf.WriteByte(c)
		}
	}
	return buf.Bytes()
}

// isAlphaNum returns true for ASCII letters and digits.
func isAlphaNum(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// foldToUpper converts lowercase ASCII to uppercase.
func foldToUpper(b []byte) []byte {
	result := make([]byte, len(b))
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			result[i] = c - 32
		} else {
			result[i] = c
		}
	}
	return result
}
