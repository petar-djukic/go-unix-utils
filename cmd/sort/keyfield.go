// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// keyfield.go implements sort key field parsing and extraction for srd053-sort R3.1-R3.4.
package main

import (
	"fmt"
	"strconv"
	"strings"
)

// keyPos represents a field.character position in a KEYDEF.
type keyPos struct {
	field int // 1-based field number
	char  int // 1-based character position; 0 means default
}

// keyOpts holds per-key modifier flags parsed from a KEYDEF.
// These control the comparison mode for the key.
type keyOpts struct {
	numeric        bool // n
	humanNumeric   bool // h
	monthSort      bool // M
	versionSort    bool // V
	reverse        bool // r
	ignoreBlanks   bool // b (used during parsing; split into startIB/endIB in keySpec)
	dictOrder      bool // d
	ignoreCase     bool // f
	ignoreNonPrint bool // i
}

// keySpec represents a parsed -k KEYDEF specification.
// R3.2: format is F[.C][OPTS][,F[.C][OPTS]].
// The b option is position-specific: startIB applies to the start position,
// endIB applies to the end position. Other opts are merged.
type keySpec struct {
	start   keyPos
	end     keyPos // end.field==0 means end of line
	opts    keyOpts
	hasOpts bool // true if any option letters were in the KEYDEF
	startIB bool // b on start position
	endIB   bool // b on end position
}

// parseKeyDef parses a KEYDEF string into a keySpec.
func parseKeyDef(s string) (keySpec, error) {
	var ks keySpec
	parts := strings.SplitN(s, ",", 2)
	field, char, opts, hasOpts, err := parseKeyPos(parts[0])
	if err != nil {
		return ks, fmt.Errorf("invalid key: %s", s)
	}
	ks.start = keyPos{field: field, char: char}
	ks.startIB = opts.ignoreBlanks
	opts.ignoreBlanks = false
	ks.opts = opts
	ks.hasOpts = hasOpts
	if len(parts) == 2 {
		return parseKeyEnd(&ks, parts[1], s)
	}
	return ks, nil
}

// parseKeyEnd parses the end portion of a KEYDEF into ks.
func parseKeyEnd(ks *keySpec, endStr, fullKey string) (keySpec, error) {
	field, char, opts, ho, err := parseKeyPos(endStr)
	if err != nil {
		return *ks, fmt.Errorf("invalid key: %s", fullKey)
	}
	ks.end = keyPos{field: field, char: char}
	ks.endIB = opts.ignoreBlanks
	opts.ignoreBlanks = false
	mergeKeyOpts(&ks.opts, &opts)
	if ho {
		ks.hasOpts = true
	}
	return *ks, nil
}

// parseKeyPos parses F[.C][OPTS] returning field, char, opts, hasOpts.
func parseKeyPos(s string) (int, int, keyOpts, bool, error) {
	var opts keyOpts
	i, field, err := parseFieldNumber(s)
	if err != nil {
		return 0, 0, opts, false, err
	}
	char := 0
	if i < len(s) && s[i] == '.' {
		i, char = parseCharOffset(s, i+1)
	}
	hasOpts := i < len(s)
	for i < len(s) {
		if !applyKeyOpt(&opts, s[i]) {
			return 0, 0, opts, false, fmt.Errorf("invalid option: %c", s[i])
		}
		i++
	}
	return field, char, opts, hasOpts, nil
}

// parseFieldNumber reads leading digits from s and returns (index-after, field-number).
func parseFieldNumber(s string) (int, int, error) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0, fmt.Errorf("missing field number")
	}
	field, _ := strconv.Atoi(s[:i])
	if field == 0 {
		return 0, 0, fmt.Errorf("field number must be positive")
	}
	return i, field, nil
}

// parseCharOffset reads digits after the dot in F.C.
func parseCharOffset(s string, start int) (int, int) {
	i := start
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == start {
		return i, 0
	}
	val, _ := strconv.Atoi(s[start:i])
	return i, val
}

// applyKeyOpt sets the corresponding flag in opts for ch. Returns false if invalid.
func applyKeyOpt(opts *keyOpts, ch byte) bool {
	switch ch {
	case 'n':
		opts.numeric = true
	case 'h':
		opts.humanNumeric = true
	case 'M':
		opts.monthSort = true
	case 'V':
		opts.versionSort = true
	case 'r':
		opts.reverse = true
	case 'b':
		opts.ignoreBlanks = true
	case 'd':
		opts.dictOrder = true
	case 'f':
		opts.ignoreCase = true
	case 'i':
		opts.ignoreNonPrint = true
	default:
		return false
	}
	return true
}

// mergeKeyOpts ORs src options into dst (excluding ignoreBlanks which is
// tracked per-position in keySpec.startIB/endIB).
func mergeKeyOpts(dst, src *keyOpts) {
	dst.numeric = dst.numeric || src.numeric
	dst.humanNumeric = dst.humanNumeric || src.humanNumeric
	dst.monthSort = dst.monthSort || src.monthSort
	dst.versionSort = dst.versionSort || src.versionSort
	dst.reverse = dst.reverse || src.reverse
	dst.dictOrder = dst.dictOrder || src.dictOrder
	dst.ignoreCase = dst.ignoreCase || src.ignoreCase
	dst.ignoreNonPrint = dst.ignoreNonPrint || src.ignoreNonPrint
}

// extractKey returns the sort key substring from line for the given key spec.
// R3.1: uses sep as field delimiter (0 means default blank separation).
// R3.4: globalIB (global -b) combined with per-position b controls blank handling.
// The b option applies only to the position it's attached to in the KEYDEF.
func extractKey(line string, ks *keySpec, sep byte, globalIB bool) string {
	sib := globalIB || ks.startIB
	start := keyStartPos(line, ks.start, sep, sib)
	if ks.end.field == 0 {
		if start >= len(line) {
			return ""
		}
		return line[start:]
	}
	eib := globalIB || ks.endIB
	end := keyEndPos(line, ks.end, sep, eib)
	return boundedSubstr(line, start, end)
}

// boundedSubstr returns line[start:end] clamped to valid bounds.
func boundedSubstr(line string, start, end int) string {
	if start >= end || start >= len(line) {
		return ""
	}
	if end > len(line) {
		end = len(line)
	}
	return line[start:end]
}

// keyStartPos returns the 0-based byte offset for the start of a key.
func keyStartPos(line string, pos keyPos, sep byte, ib bool) int {
	idx := fieldBegin(line, pos.field, sep)
	if ib {
		idx = skipBlanks(line, idx)
	}
	c := pos.char
	if c == 0 {
		c = 1
	}
	return idx + c - 1
}

// keyEndPos returns the 0-based byte offset one past the end of a key.
func keyEndPos(line string, pos keyPos, sep byte, ib bool) int {
	if pos.char == 0 {
		return fieldEnd(line, pos.field, sep)
	}
	idx := fieldBegin(line, pos.field, sep)
	if ib {
		idx = skipBlanks(line, idx)
	}
	return idx + pos.char
}

// fieldBegin returns the 0-based index of the start of field n (1-based).
func fieldBegin(line string, n int, sep byte) int {
	if sep != 0 {
		return fieldBeginSep(line, n, sep)
	}
	return fieldBeginBlanks(line, n)
}

// fieldBeginSep returns the start of field n with an explicit separator.
func fieldBeginSep(line string, n int, sep byte) int {
	if n <= 1 {
		return 0
	}
	count := 0
	for i := 0; i < len(line); i++ {
		if line[i] == sep {
			count++
			if count == n-1 {
				return i + 1
			}
		}
	}
	return len(line)
}

// fieldBeginBlanks returns the start of field n with default blank separation.
// Fields are separated by blank-to-non-blank transitions; leading blanks
// belong to the field.
func fieldBeginBlanks(line string, n int) int {
	if n <= 1 {
		return 0
	}
	i := skipBlanks(line, 0)
	fieldNum := 1
	for i < len(line) && fieldNum < n {
		for i < len(line) && !isBlank(line[i]) {
			i++
		}
		fieldNum++
		if fieldNum == n {
			return i
		}
		i = skipBlanks(line, i)
	}
	return len(line)
}

// fieldEnd returns the 0-based index one past the end of field n (1-based).
func fieldEnd(line string, n int, sep byte) int {
	if sep != 0 {
		return fieldEndSep(line, n, sep)
	}
	return fieldEndBlanks(line, n)
}

// fieldEndSep returns one past the end of field n with explicit separator.
func fieldEndSep(line string, n int, sep byte) int {
	count := 0
	for i := 0; i < len(line); i++ {
		if line[i] == sep {
			count++
			if count == n {
				return i
			}
		}
	}
	return len(line)
}

// fieldEndBlanks returns one past the end of field n with blank separation.
func fieldEndBlanks(line string, n int) int {
	start := fieldBeginBlanks(line, n)
	if start >= len(line) {
		return len(line)
	}
	i := skipBlanks(line, start)
	for i < len(line) && !isBlank(line[i]) {
		i++
	}
	return i
}

// skipBlanks advances pos past any blank characters (space or tab).
func skipBlanks(line string, pos int) int {
	for pos < len(line) && isBlank(line[pos]) {
		pos++
	}
	return pos
}

// isBlank reports whether c is a blank character (space or tab).
func isBlank(c byte) bool {
	return c == ' ' || c == '\t'
}
