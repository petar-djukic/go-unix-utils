// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-sort R3.2 key modifiers: comparison modes (h, M, V)
// and text transforms (b, d, f, i) for sort key processing.
package main

import (
	"strconv"
	"strings"
)

// months maps three-letter month abbreviations to sort order (R3.2, M modifier).
// Unknown strings map to 0 and sort before JAN.
var months = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4,
	"MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8,
	"SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

// siMultiplier maps SI suffixes to their multipliers for human-numeric sort.
// R3.2: h modifier uses powers of 1024 to match GNU sort -h behavior.
var siMultiplier = map[byte]float64{
	'K': 1024,
	'M': 1024 * 1024,
	'G': 1024 * 1024 * 1024,
	'T': 1024 * 1024 * 1024 * 1024,
	'P': 1024 * 1024 * 1024 * 1024 * 1024,
	'E': 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	'Z': 1024 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
	'Y': 1024 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
}

// transformKey applies b, d, i, f modifiers to a key string (R3.2).
// b: trim leading blanks. d: dictionary order. i: ignore non-printing. f: fold case.
func transformKey(s string, k sortKey, opts options) string {
	if k.ignoreBlanks || (!k.hasOpts && opts.ignoreBlanks) {
		s = strings.TrimLeft(s, " \t")
	}
	if k.dictOrder {
		s = filterDict(s)
	}
	if k.ignoreNonPrint {
		s = filterPrintable(s)
	}
	if k.foldCase {
		s = strings.ToUpper(s)
	}
	return s
}

// filterDict keeps only blanks and alphanumeric characters (R3.2, d modifier).
func filterDict(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == ' ' || ch == '\t' ||
			(ch >= 'a' && ch <= 'z') ||
			(ch >= 'A' && ch <= 'Z') ||
			(ch >= '0' && ch <= '9') {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// filterPrintable keeps only characters with ASCII values 32–126 (R3.2, i modifier).
func filterPrintable(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] >= 32 && s[i] <= 126 {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// compareByMode selects the comparison function based on key modifiers.
// R3.2: h=human-numeric, M=month, V=version, n=numeric, default=lexicographic.
func compareByMode(a, b string, k sortKey, opts options) int {
	switch {
	case k.humanNumeric:
		return compareHumanNumeric(a, b)
	case k.monthSort:
		return compareMonth(a, b)
	case k.versionSort:
		return compareVersion(a, b)
	case k.numeric || (!k.hasOpts && opts.numeric):
		return compareNumeric(a, b)
	default:
		return strings.Compare(a, b)
	}
}

// compareHumanNumeric compares two strings by numeric value with SI suffixes.
// R3.2: h modifier (K, M, G, T, P, E, Z, Y).
func compareHumanNumeric(a, b string) int {
	va := parseHumanNumeric(a)
	vb := parseHumanNumeric(b)
	switch {
	case va < vb:
		return -1
	case va > vb:
		return 1
	default:
		return 0
	}
}

// parseHumanNumeric extracts a numeric value with optional SI suffix.
func parseHumanNumeric(s string) float64 {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return 0
	}
	f, end := parseNumericSpan(s)
	if end < len(s) {
		if mult, ok := siMultiplier[s[end]]; ok {
			return f * mult
		}
	}
	return f
}

// parseNumericSpan parses the leading numeric portion of s and returns
// the value and the index where the numeric part ends.
func parseNumericSpan(s string) (float64, int) {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	hasDigit := false
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		hasDigit = true
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			hasDigit = true
			i++
		}
	}
	if !hasDigit {
		return 0, 0
	}
	f, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return 0, 0
	}
	return f, i
}

// compareMonth compares two strings by month abbreviation (R3.2, M modifier).
// Unknown strings sort before JAN.
func compareMonth(a, b string) int {
	ma := monthIndex(a)
	mb := monthIndex(b)
	switch {
	case ma < mb:
		return -1
	case ma > mb:
		return 1
	default:
		return 0
	}
}

// monthIndex returns the 1-based month number for a string, or 0 if unknown.
func monthIndex(s string) int {
	s = strings.TrimLeft(s, " \t")
	if len(s) < 3 {
		return 0
	}
	abbr := strings.ToUpper(s[:3])
	if m, ok := months[abbr]; ok {
		return m
	}
	return 0
}

// compareVersion compares two strings using version-number sorting (R3.2, V modifier).
// Digit runs are compared numerically; non-digit chars are compared by byte value.
func compareVersion(a, b string) int {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		aDigit := a[ai] >= '0' && a[ai] <= '9'
		bDigit := b[bi] >= '0' && b[bi] <= '9'
		if aDigit && bDigit {
			cmp, na, nb := cmpDigitRun(a, ai, b, bi)
			if cmp != 0 {
				return cmp
			}
			ai, bi = na, nb
			continue
		}
		if a[ai] != b[bi] {
			return byteCmp(a[ai], b[bi])
		}
		ai++
		bi++
	}
	return intCmp(len(a), len(b))
}

// cmpDigitRun compares digit runs starting at ai and bi numerically.
// Returns comparison result and new indices past each digit run.
func cmpDigitRun(a string, ai int, b string, bi int) (int, int, int) {
	ae := scanDigits(a, ai)
	be := scanDigits(b, bi)
	as := skipLeadingZeros(a, ai, ae)
	bs := skipLeadingZeros(b, bi, be)
	aLen, bLen := ae-as, be-bs
	if aLen != bLen {
		return intCmp(aLen, bLen), ae, be
	}
	for k := 0; k < aLen; k++ {
		if a[as+k] != b[bs+k] {
			return byteCmp(a[as+k], b[bs+k]), ae, be
		}
	}
	return 0, ae, be
}

// scanDigits returns the index past the last digit starting at pos.
func scanDigits(s string, pos int) int {
	for pos < len(s) && s[pos] >= '0' && s[pos] <= '9' {
		pos++
	}
	return pos
}

// skipLeadingZeros returns the index of the first non-zero digit,
// keeping at least one digit (the last one).
func skipLeadingZeros(s string, start, end int) int {
	for start < end-1 && s[start] == '0' {
		start++
	}
	return start
}

// byteCmp returns -1, 0, or 1 comparing two bytes.
func byteCmp(a, b byte) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// intCmp returns -1, 0, or 1 comparing two ints.
func intCmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
