// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

func reverseSlice(s []string) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

func strverscmp(a, b string) int {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		ca, cb := a[i], b[j]
		if isDigit(ca) && isDigit(cb) {
			cmp := compareDigitRun(a, b, &i, &j)
			if cmp != 0 {
				return cmp
			}
			continue
		}
		if ca != cb {
			return int(ca) - int(cb)
		}
		i++
		j++
	}
	return len(a) - len(b)
}

func compareDigitRun(a, b string, i, j *int) int {
	if a[*i] == '0' || b[*j] == '0' {
		return compareLeadingZeros(a, b, i, j)
	}
	return compareNumeric(a, b, i, j)
}

func compareLeadingZeros(a, b string, i, j *int) int {
	for *i < len(a) && *j < len(b) {
		ca, cb := a[*i], b[*j]
		aDigit, bDigit := isDigit(ca), isDigit(cb)
		if !aDigit && !bDigit {
			return 0
		}
		if !aDigit {
			return -1
		}
		if !bDigit {
			return 1
		}
		if ca != cb {
			return int(ca) - int(cb)
		}
		*i++
		*j++
	}
	if *i < len(a) && isDigit(a[*i]) {
		return 1
	}
	if *j < len(b) && isDigit(b[*j]) {
		return -1
	}
	return 0
}

func compareNumeric(a, b string, i, j *int) int {
	ai, aj := *i, *j
	for ai < len(a) && isDigit(a[ai]) {
		ai++
	}
	for aj < len(b) && isDigit(b[aj]) {
		aj++
	}
	lenA, lenB := ai-*i, aj-*j
	if lenA != lenB {
		*i = ai
		*j = aj
		return lenA - lenB
	}
	for *i < ai {
		if a[*i] != b[*j] {
			result := int(a[*i]) - int(b[*j])
			*i = ai
			*j = aj
			return result
		}
		*i++
		*j++
	}
	return 0
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}
