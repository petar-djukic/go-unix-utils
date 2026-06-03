// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"strconv"
	"strings"
)

func cmpFloat(parse func(string) float64) func(string, string) int {
	return func(a, b string) int {
		na, nb := parse(a), parse(b)
		if na < nb {
			return -1
		}
		if na > nb {
			return 1
		}
		return 0
	}
}

func floatCmp(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func parseNumericEnd(s string) (float64, int) {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	start := i
	if i < len(s) && (s[i] == '-' || s[i] == '+') {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	if i == start || (i == start+1 && (s[start] == '-' || s[start] == '+')) {
		return 0, i
	}
	val, err := strconv.ParseFloat(s[start:i], 64)
	if err != nil {
		return 0, i
	}
	return val, i
}

func parseNumeric(s string) float64 {
	val, _ := parseNumericEnd(s)
	return val
}

func parseHumanNumeric(s string) float64 {
	val, end := parseNumericEnd(s)
	if end < len(s) {
		if m := suffixMultiplier(s[end]); m > 0 {
			return val * m
		}
	}
	return val
}

func suffixMultiplier(c byte) float64 {
	switch c {
	case 'K', 'k':
		return 1 << 10
	case 'M':
		return 1 << 20
	case 'G':
		return 1 << 30
	case 'T':
		return 1 << 40
	case 'P':
		return 1 << 50
	case 'E':
		return 1 << 60
	case 'Z':
		return 1 << 70
	case 'Y':
		return 1 << 80
	default:
		return 0
	}
}

func monthRank(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	end := min(i+3, len(s))
	return monthMap[strings.ToUpper(s[i:end])]
}

func compareVersion(a, b string) int {
	ia, ib := 0, 0
	for ia < len(a) && ib < len(b) {
		da := a[ia] >= '0' && a[ia] <= '9'
		db := b[ib] >= '0' && b[ib] <= '9'
		if da && db {
			r := cmpNumSegments(a, b, &ia, &ib)
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

func cmpNumSegments(a, b string, ia, ib *int) int {
	startA, startB := *ia, *ib
	for *ia < len(a) && a[*ia] == '0' {
		*ia++
	}
	for *ib < len(b) && b[*ib] == '0' {
		*ib++
	}
	sigA, sigB := *ia, *ib
	endA := sigA
	for endA < len(a) && a[endA] >= '0' && a[endA] <= '9' {
		endA++
	}
	endB := sigB
	for endB < len(b) && b[endB] >= '0' && b[endB] <= '9' {
		endB++
	}
	lenA, lenB := endA-sigA, endB-sigB
	*ia = endA
	*ib = endB
	if lenA != lenB {
		return lenA - lenB
	}
	for k := range lenA {
		if a[sigA+k] != b[sigB+k] {
			return int(a[sigA+k]) - int(b[sigB+k])
		}
	}
	return (sigB - startB) - (sigA - startA)
}

func modeCompare(a, b string, mode sortMode) int {
	switch mode {
	case sortNumeric:
		return floatCmp(parseNumeric(a), parseNumeric(b))
	case sortHumanNumeric:
		return floatCmp(parseHumanNumeric(a), parseHumanNumeric(b))
	case sortMonth:
		return monthRank(a) - monthRank(b)
	case sortVersion:
		return compareVersion(a, b)
	default:
		return strings.Compare(a, b)
	}
}
