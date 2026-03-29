// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Key field sorting for cmd/sort.
//
// Implements prd053-sort R3.1 (field separator), R3.2 (key fields),
// R3.3 (multiple keys), R3.4 (ignore leading blanks).
package main

import (
	"fmt"
	"strconv"
	"strings"
)

// keyOpts holds per-key modifier options parsed from KEYDEF OPTS.
// R3.2: modifier letters (n, r, h, M, V, b, d, f, i).
type keyOpts struct {
	mode           sortMode
	hasMode        bool
	reverse        bool
	ignoreBlanks   bool
	dictOrder      bool
	foldCase       bool
	ignoreNonPrint bool
}

// keySpec defines a sort key with start/end positions and options.
// R3.2: KEYDEF format is F[.C][OPTS][,F[.C][OPTS]].
type keySpec struct {
	startField int // 1-based field number
	startChar  int // 1-based char position, 0 = start of field
	endField   int // 1-based, 0 = through end of line
	endChar    int // 1-based, 0 = through end of field
	opts       keyOpts
}

// fieldRange represents the byte range [start, end) of a field in a line.
type fieldRange struct {
	start int
	end   int
}

// parseKeyDef parses a KEYDEF string into a keySpec.
// R3.2: format is F[.C][OPTS][,F[.C][OPTS]].
func parseKeyDef(s string) (keySpec, error) {
	parts := strings.SplitN(s, ",", 2)
	sf, sc, sopts, err := parseKeyPos(parts[0])
	if err != nil {
		return keySpec{}, fmt.Errorf("invalid key: %s: %w", s, err)
	}
	k := keySpec{startField: sf, startChar: sc, opts: sopts}
	if len(parts) == 2 {
		var ef, ec int
		var eopts keyOpts
		ef, ec, eopts, err = parseKeyPos(parts[1])
		if err != nil {
			return keySpec{}, fmt.Errorf("invalid key: %s: %w", s, err)
		}
		k.endField = ef
		k.endChar = ec
		k.opts = mergeKeyOpts(sopts, eopts)
	}
	return k, nil
}

// parseKeyPos parses F[.C][OPTS] returning field, char, and options.
func parseKeyPos(s string) (int, int, keyOpts, error) {
	fieldStr, idx := consumeDigits(s, 0)
	if fieldStr == "" {
		return 0, 0, keyOpts{}, fmt.Errorf("missing field number")
	}
	field, _ := strconv.Atoi(fieldStr)
	char := 0
	if idx < len(s) && s[idx] == '.' {
		var charStr string
		charStr, idx = consumeDigits(s, idx+1)
		if charStr != "" {
			char, _ = strconv.Atoi(charStr)
		}
	}
	opts, err := parseKeyModifiers(s[idx:])
	if err != nil {
		return 0, 0, keyOpts{}, err
	}
	return field, char, opts, nil
}

// parseKeyModifiers parses modifier letters into keyOpts.
func parseKeyModifiers(s string) (keyOpts, error) {
	var o keyOpts
	for _, c := range s {
		switch c {
		case 'b':
			o.ignoreBlanks = true
		case 'd':
			o.dictOrder = true
		case 'f':
			o.foldCase = true
		case 'i':
			o.ignoreNonPrint = true
		case 'n':
			o.mode, o.hasMode = modeNumeric, true
		case 'h':
			o.mode, o.hasMode = modeHumanNumeric, true
		case 'M':
			o.mode, o.hasMode = modeMonth, true
		case 'r':
			o.reverse = true
		case 'V':
			o.mode, o.hasMode = modeVersion, true
		default:
			return keyOpts{}, fmt.Errorf("invalid key option: %c", c)
		}
	}
	return o, nil
}

// mergeKeyOpts combines start and end position options.
func mergeKeyOpts(start, end keyOpts) keyOpts {
	if end.hasMode {
		start.mode, start.hasMode = end.mode, true
	}
	if end.reverse {
		start.reverse = true
	}
	if end.ignoreBlanks {
		start.ignoreBlanks = true
	}
	if end.dictOrder {
		start.dictOrder = true
	}
	if end.foldCase {
		start.foldCase = true
	}
	if end.ignoreNonPrint {
		start.ignoreNonPrint = true
	}
	return start
}

// consumeDigits reads consecutive digits starting at from.
func consumeDigits(s string, from int) (string, int) {
	i := from
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[from:i], i
}

// compareByKeys compares two lines using all key specs in order.
// R3.3: earlier keys take precedence; later keys break ties.
func compareByKeys(a, b string, opts sortOptions) int {
	for _, k := range opts.keys {
		ka := extractKey(a, k, opts.fieldSep, opts.ignoreBlanks)
		kb := extractKey(b, k, opts.fieldSep, opts.ignoreBlanks)
		ka = transformKey(ka, k.opts)
		kb = transformKey(kb, k.opts)
		mode := resolveKeyMode(k.opts, opts.mode)
		cmp := compareLine(ka, kb, mode)
		if cmp != 0 {
			if k.opts.reverse {
				return -cmp
			}
			return cmp
		}
	}
	return 0
}

// resolveKeyMode returns the per-key mode if set, else the global mode.
func resolveKeyMode(ko keyOpts, globalMode sortMode) sortMode {
	if ko.hasMode {
		return ko.mode
	}
	return globalMode
}

// extractKey extracts the sort key substring from a line.
func extractKey(line string, k keySpec, sep string, globalBlanks bool) string {
	fields := getFields(line, sep)
	ib := k.opts.ignoreBlanks || globalBlanks
	startByte := keyStartByte(line, fields, k.startField, k.startChar, ib)
	endByte := keyEndByte(line, fields, k.endField, k.endChar, ib)
	if startByte >= len(line) || startByte >= endByte {
		return ""
	}
	return line[startByte:endByte]
}

// getFields splits a line into field ranges.
// R3.1: with sep, uses CHAR as delimiter; without, uses blank-to-non-blank.
func getFields(line string, sep string) []fieldRange {
	if sep != "" {
		return getFieldsSep(line, sep[0])
	}
	return getFieldsDefault(line)
}

// getFieldsSep splits a line by a single separator character.
func getFieldsSep(line string, sep byte) []fieldRange {
	var fields []fieldRange
	start := 0
	for i := 0; i < len(line); i++ {
		if line[i] == sep {
			fields = append(fields, fieldRange{start, i})
			start = i + 1
		}
	}
	fields = append(fields, fieldRange{start, len(line)})
	return fields
}

// getFieldsDefault splits a line by blank-to-non-blank transitions.
func getFieldsDefault(line string) []fieldRange {
	var fields []fieldRange
	i := 0
	for i < len(line) {
		fieldStart := i
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			i++
		}
		fields = append(fields, fieldRange{fieldStart, i})
	}
	if len(fields) == 0 {
		fields = append(fields, fieldRange{0, 0})
	}
	return fields
}

// keyStartByte computes the starting byte offset for a key's start position.
func keyStartByte(
	line string, fields []fieldRange, field, char int, ib bool,
) int {
	if field < 1 || field > len(fields) {
		return len(line)
	}
	f := fields[field-1]
	pos := f.start
	if ib {
		pos = skipBlanksFrom(line, pos, f.end)
	}
	if char > 1 {
		pos += char - 1
	}
	if pos > len(line) {
		return len(line)
	}
	return pos
}

// keyEndByte computes the ending byte offset (exclusive) for a key's end.
func keyEndByte(
	line string, fields []fieldRange, field, char int, ib bool,
) int {
	if field == 0 {
		return len(line)
	}
	if field < 1 || field > len(fields) {
		return len(line)
	}
	f := fields[field-1]
	if char == 0 {
		return f.end
	}
	pos := f.start
	if ib {
		pos = skipBlanksFrom(line, pos, f.end)
	}
	pos += char
	if pos > len(line) {
		return len(line)
	}
	return pos
}

// skipBlanksFrom skips blanks starting at from, bounded by limit.
func skipBlanksFrom(line string, from, limit int) int {
	i := from
	for i < limit && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return i
}

// transformKey applies d, f, i modifiers to a key string for comparison.
func transformKey(s string, opts keyOpts) string {
	if !opts.dictOrder && !opts.foldCase && !opts.ignoreNonPrint {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if opts.ignoreNonPrint && (c < 0x20 || c > 0x7E) {
			continue
		}
		if opts.dictOrder && !isAlphaNum(c) && c != ' ' && c != '\t' {
			continue
		}
		if opts.foldCase && c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

// isAlphaNum reports whether b is an ASCII letter or digit.
func isAlphaNum(b byte) bool {
	return isDigit(b) || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
