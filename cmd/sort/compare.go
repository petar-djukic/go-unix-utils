// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// compare.go implements sort comparison functions for srd053-sort R2.1-R2.4, R3.1-R3.4.
package main

import (
	"strconv"
	"strings"
)

// compareFunc returns a three-way comparison function for the active sort mode.
// R2.1: -n numeric. R2.2: -h human-numeric. R2.3: -M month. R2.4: -V version.
func compareFunc(cfg *config) func(string, string) int {
	if cfg.numericSort {
		return compareNumeric
	}
	if cfg.humanNumeric {
		return compareHumanNumeric
	}
	if cfg.monthSort {
		return compareMonth
	}
	if cfg.versionSort {
		return compareVersion
	}
	return strings.Compare
}

// compareKeys compares two lines using the parsed key specifications.
// R3.2, R3.3: earlier keys take precedence; later keys break ties.
func compareKeys(a, b string, cfg *config) int {
	for k := range cfg.parsedKeys {
		ks := &cfg.parsedKeys[k]
		ka := extractKey(a, ks, cfg.sepByte, cfg.ignoreBlanks)
		kb := extractKey(b, ks, cfg.sepByte, cfg.ignoreBlanks)
		cmpFn := keySortFunc(ks, cfg)
		result := cmpFn(ka, kb)
		if effectiveReverse(ks, cfg) {
			result = -result
		}
		if result != 0 {
			return result
		}
	}
	return 0
}

// keySortFunc returns the comparison function for a specific key.
// If the key has its own opts, uses those; otherwise uses the global sort mode.
func keySortFunc(ks *keySpec, cfg *config) func(string, string) int {
	if ks.hasOpts {
		return optsCompareFunc(&ks.opts)
	}
	return compareFunc(cfg)
}

// optsCompareFunc returns a comparison function for the given key options.
func optsCompareFunc(opts *keyOpts) func(string, string) int {
	if opts.numeric {
		return compareNumeric
	}
	if opts.humanNumeric {
		return compareHumanNumeric
	}
	if opts.monthSort {
		return compareMonth
	}
	if opts.versionSort {
		return compareVersion
	}
	if opts.ignoreCase {
		return compareFoldCase
	}
	return strings.Compare
}

// compareFoldCase compares strings case-insensitively.
func compareFoldCase(a, b string) int {
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

// effectiveReverse returns whether this key's comparison should be reversed.
// Per-key opts override global -r when present.
func effectiveReverse(ks *keySpec, cfg *config) bool {
	if ks.hasOpts {
		return ks.opts.reverse
	}
	return cfg.reverse
}

// compareNumeric compares two strings by their leading numeric value.
// R2.1: parses leading whitespace and optional sign.
func compareNumeric(a, b string) int {
	va := parseNumeric(a)
	vb := parseNumeric(b)
	return compareFloat(va, vb)
}

// parseNumeric extracts the leading numeric value from s.
// Handles leading whitespace, optional sign, digits, and decimal point.
func parseNumeric(s string) float64 {
	s = strings.TrimLeft(s, " \t")
	end := numericPrefixLen(s)
	if end == 0 {
		return 0
	}
	val, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		return 0
	}
	return val
}

// numericPrefixLen returns the length of the leading numeric prefix in s.
// Recognizes optional sign, digits, optional decimal point and more digits.
func numericPrefixLen(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	hasDigit := false
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		hasDigit = true
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			hasDigit = true
		}
	}
	if !hasDigit {
		return 0
	}
	return i
}

// compareFloat returns -1, 0, or 1 comparing two float64 values.
func compareFloat(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// compareHumanNumeric compares strings by numeric value with SI suffixes.
// R2.2: suffix rank dominates; higher suffix means larger value.
func compareHumanNumeric(a, b string) int {
	va, ra := parseHumanParts(a)
	vb, rb := parseHumanParts(b)
	if va == 0 && vb == 0 {
		return 0
	}
	if va < 0 && vb >= 0 {
		return -1
	}
	if va >= 0 && vb < 0 {
		return 1
	}
	if ra != rb {
		diff := ra - rb
		if va < 0 {
			return -diff
		}
		return diff
	}
	return compareFloat(va, vb)
}

// parseHumanParts extracts the numeric value and suffix rank from s.
func parseHumanParts(s string) (float64, int) {
	s = strings.TrimLeft(s, " \t")
	end := numericPrefixLen(s)
	if end == 0 {
		return 0, 0
	}
	val, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		return 0, 0
	}
	rank := 0
	if end < len(s) {
		rank = suffixRank(s[end])
	}
	return val, rank
}

// suffixRank maps SI suffix characters to their rank for comparison.
// R2.2: K < M < G < T < P < E < Z < Y.
func suffixRank(b byte) int {
	switch b {
	case 'K', 'k':
		return 1
	case 'M':
		return 2
	case 'G':
		return 3
	case 'T':
		return 4
	case 'P':
		return 5
	case 'E':
		return 6
	case 'Z':
		return 7
	case 'Y':
		return 8
	}
	return 0
}

// monthMap maps uppercase 3-letter month abbreviations to their sort order.
var monthMap = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4,
	"MAY": 5, "JUN": 6, "JUL": 7, "AUG": 8,
	"SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

// compareMonth compares strings by month name abbreviation.
// R2.3: unknown strings sort before JAN (rank 0).
func compareMonth(a, b string) int {
	return monthRank(a) - monthRank(b)
}

// monthRank extracts the month rank from the leading 3 characters.
func monthRank(s string) int {
	s = strings.TrimLeft(s, " \t")
	if len(s) < 3 {
		return 0
	}
	key := strings.ToUpper(s[:3])
	if rank, ok := monthMap[key]; ok {
		return rank
	}
	return 0
}

// compareVersion compares strings using version number sorting.
// R2.4: numeric segments are compared as integers; non-numeric as bytes.
func compareVersion(a, b string) int {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		if isDigit(a[ia]) && isDigit(b[ib]) {
			r := cmpDigitRun(a, b, &ia, &ib)
			if r != 0 {
				return r
			}
			continue
		}
		if a[ia] != b[ib] {
			return int(a[ia]) - int(b[ib])
		}
		ia++
		ib++
	}
	return len(a) - len(b)
}

// cmpDigitRun compares two digit runs, advancing indices past them.
// Leading-zero runs are compared as fractions; others as integers.
func cmpDigitRun(a, b string, ia, ib *int) int {
	sa := extractDigits(a, ia)
	sb := extractDigits(b, ib)
	if hasLeadingZero(sa) || hasLeadingZero(sb) {
		return cmpFractional(sa, sb)
	}
	na := strings.TrimLeft(sa, "0")
	nb := strings.TrimLeft(sb, "0")
	if len(na) != len(nb) {
		return len(na) - len(nb)
	}
	return strings.Compare(na, nb)
}

// hasLeadingZero reports whether s has more than one digit starting with '0'.
func hasLeadingZero(s string) bool {
	return len(s) > 1 && s[0] == '0'
}

// cmpFractional compares two digit strings left-aligned (fractional mode).
func cmpFractional(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) {
		if a[i] != b[i] {
			return int(a[i]) - int(b[i])
		}
		i++
	}
	return len(a) - len(b)
}

// extractDigits extracts a run of digits from s starting at *idx.
func extractDigits(s string, idx *int) string {
	start := *idx
	for *idx < len(s) && isDigit(s[*idx]) {
		(*idx)++
	}
	return s[start:*idx]
}

// isDigit reports whether c is an ASCII digit.
func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
