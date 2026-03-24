// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd053-sort R3.1–R3.3:
// field separator (-t), key definitions (-k), and key-based comparison.
package main

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// sortKey represents a parsed -k key definition.
// R3.2: KEYDEF format is F[.C][OPTS][,F[.C][OPTS]].
type sortKey struct {
	startField int     // 1-based field number
	startChar  int     // 1-based char within field (default 1)
	endField   int     // 1-based field number (0 = through end of line)
	endChar    int     // 1-based char within field (0 = end of field)
	mods       keyMods // key-local modifier flags
}

// keyMods holds modifier flags parsed from key position suffixes.
type keyMods struct {
	blanks     bool // b - ignore leading blanks
	dict       bool // d - dictionary order
	foldCase   bool // f - fold lower to upper
	generalNum bool // g - general numeric
	humanNum   bool // h - human numeric
	ignoreNP   bool // i - ignore nonprinting
	numeric    bool // n - numeric sort
	reverse    bool // r - reverse
	version    bool // V - version sort
}

// hasAnyMod returns true if any modifier is set.
func (m keyMods) hasAnyMod() bool {
	return m.blanks || m.dict || m.foldCase || m.generalNum ||
		m.humanNum || m.ignoreNP || m.numeric || m.reverse || m.version
}

// --- Key definition parsing ---

// parseKeyDef parses a -k KEYDEF string.
// R3.2: Format is F[.C][OPTS][,F[.C][OPTS]].
func parseKeyDef(s string) (sortKey, error) {
	parts := strings.SplitN(s, ",", 2)
	sf, sc, sm, err := parseKeyPos(parts[0])
	if err != nil {
		return sortKey{}, fmt.Errorf("invalid key: %s: %w", s, err)
	}
	key := sortKey{startField: sf, startChar: sc, mods: sm}
	if key.startChar == 0 {
		key.startChar = 1 // R3.2: default start char is 1
	}
	if len(parts) == 2 {
		return parseKeyEnd(key, parts[1], s)
	}
	return key, nil
}

// parseKeyEnd parses the end position of a key definition.
func parseKeyEnd(key sortKey, endStr, full string) (sortKey, error) {
	ef, ec, em, err := parseKeyPos(endStr)
	if err != nil {
		return sortKey{}, fmt.Errorf("invalid key: %s: %w", full, err)
	}
	key.endField = ef
	key.endChar = ec
	mergeMods(&key.mods, em)
	return key, nil
}

// parseKeyPos parses F[.C][OPTS] returning field, char, and modifiers.
func parseKeyPos(s string) (int, int, keyMods, error) {
	i := scanDigits(s, 0)
	if i == 0 {
		return 0, 0, keyMods{}, fmt.Errorf("missing field number")
	}
	field, _ := strconv.Atoi(s[:i])
	charPos := 0
	if i < len(s) && s[i] == '.' {
		i++
		cStart := i
		i = scanDigits(s, i)
		if i > cStart {
			charPos, _ = strconv.Atoi(s[cStart:i])
		}
	}
	mods, err := parseModifiers(s[i:])
	if err != nil {
		return 0, 0, keyMods{}, err
	}
	return field, charPos, mods, nil
}

// scanDigits returns the index past consecutive digits starting at pos.
func scanDigits(s string, pos int) int {
	i := pos
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return i
}

// parseModifiers parses modifier letter string into keyMods.
func parseModifiers(s string) (keyMods, error) {
	var m keyMods
	for _, ch := range s {
		if err := applyModifier(&m, ch); err != nil {
			return keyMods{}, err
		}
	}
	return m, nil
}

// applyModifier sets a single modifier flag.
func applyModifier(m *keyMods, ch rune) error {
	switch ch {
	case 'b':
		m.blanks = true
	case 'd':
		m.dict = true
	case 'f':
		m.foldCase = true
	case 'g':
		m.generalNum = true
	case 'h':
		m.humanNum = true
	case 'i':
		m.ignoreNP = true
	case 'n':
		m.numeric = true
	case 'r':
		m.reverse = true
	case 'V':
		m.version = true
	default:
		return fmt.Errorf("invalid modifier: %c", ch)
	}
	return nil
}

// mergeMods combines modifiers from the end position into dst.
func mergeMods(dst *keyMods, src keyMods) {
	if src.blanks {
		dst.blanks = true
	}
	if src.dict {
		dst.dict = true
	}
	if src.foldCase {
		dst.foldCase = true
	}
	if src.generalNum {
		dst.generalNum = true
	}
	if src.humanNum {
		dst.humanNum = true
	}
	if src.ignoreNP {
		dst.ignoreNP = true
	}
	if src.numeric {
		dst.numeric = true
	}
	if src.reverse {
		dst.reverse = true
	}
	if src.version {
		dst.version = true
	}
}

// --- Field extraction ---

// extractKey extracts the sort key substring from a line.
// R3.2: key field extraction with character offsets.
func extractKey(line []byte, key sortKey, sep string) []byte {
	startIdx := keyStartIndex(line, key, sep)
	endIdx := keyEndIndex(line, key, sep)
	if startIdx >= endIdx {
		return nil
	}
	return line[startIdx:endIdx]
}

// keyStartIndex returns the byte offset where the key begins.
func keyStartIndex(line []byte, key sortKey, sep string) int {
	idx := fieldStart(line, key.startField, sep)
	if key.startChar > 1 {
		idx += key.startChar - 1
	}
	return clampIndex(idx, len(line))
}

// keyEndIndex returns the byte offset where the key ends (exclusive).
func keyEndIndex(line []byte, key sortKey, sep string) int {
	if key.endField == 0 {
		return len(line)
	}
	if key.endChar > 0 {
		idx := fieldStart(line, key.endField, sep) + key.endChar
		return clampIndex(idx, len(line))
	}
	return clampIndex(fieldEnd(line, key.endField, sep), len(line))
}

// clampIndex ensures idx does not exceed max.
func clampIndex(idx, max int) int {
	if idx > max {
		return max
	}
	return idx
}

// fieldStart returns the byte index where field n starts (1-based).
func fieldStart(line []byte, n int, sep string) int {
	if sep != "" {
		return fieldStartSep(line, n, sep[0])
	}
	return fieldStartDefault(line, n)
}

// fieldEnd returns the byte index past the end of field n.
func fieldEnd(line []byte, n int, sep string) int {
	if sep != "" {
		return fieldEndSep(line, n, sep[0])
	}
	return fieldEndDefault(line, n)
}

// fieldStartDefault finds field start with blank-to-non-blank transitions.
// R3.1: default separator is blank-to-non-blank transition.
func fieldStartDefault(line []byte, n int) int {
	i := 0
	curField := 1
	for i < len(line) && isBlank(line[i]) {
		i++
	}
	for curField < n && i < len(line) {
		for i < len(line) && !isBlank(line[i]) {
			i++
		}
		for i < len(line) && isBlank(line[i]) {
			i++
		}
		curField++
	}
	return i
}

// fieldEndDefault finds field end with default separator.
func fieldEndDefault(line []byte, n int) int {
	i := fieldStartDefault(line, n)
	for i < len(line) && !isBlank(line[i]) {
		i++
	}
	return i
}

// fieldStartSep finds field start with explicit separator.
// R3.1: -t CHAR uses exact character delimiter.
func fieldStartSep(line []byte, n int, sep byte) int {
	field := 1
	i := 0
	for field < n && i < len(line) {
		if line[i] == sep {
			field++
		}
		i++
	}
	if field < n {
		return len(line)
	}
	return i
}

// fieldEndSep finds field end with explicit separator.
func fieldEndSep(line []byte, n int, sep byte) int {
	start := fieldStartSep(line, n, sep)
	i := start
	for i < len(line) && line[i] != sep {
		i++
	}
	return i
}

// isBlank returns true for space or tab characters.
func isBlank(b byte) bool {
	return b == ' ' || b == '\t'
}

// --- Comparison ---

// makeCompare returns the less-than function for sorting.
// D2: When no -k is specified, the entire line is the sort key.
func makeCompare(cfg config) func(a, b []byte) bool {
	if len(cfg.keys) > 0 {
		return func(a, b []byte) bool {
			return compareKeys(a, b, cfg) < 0
		}
	}
	return selectCompare(cfg)
}

// makeEqual returns the equality function for deduplication.
func makeEqual(cfg config) func(a, b []byte) bool {
	if len(cfg.keys) > 0 {
		return func(a, b []byte) bool {
			return keysEqual(a, b, cfg)
		}
	}
	return selectEqual(cfg)
}

// compareKeys compares two lines using all configured keys.
// D3: keys evaluated left to right; fall back to whole-line if all equal.
func compareKeys(a, b []byte, cfg config) int {
	for _, key := range cfg.keys {
		ka := extractKey(a, key, cfg.separator)
		kb := extractKey(b, key, cfg.separator)
		mods := effectiveMods(key.mods, cfg)
		cmp := compareByMods(ka, kb, mods)
		if mods.reverse {
			cmp = -cmp
		}
		if cmp != 0 {
			return cmp
		}
	}
	if cfg.stable {
		return 0
	}
	return lastResortCompare(a, b, cfg)
}

// lastResortCompare performs whole-line comparison as final tiebreaker.
func lastResortCompare(a, b []byte, cfg config) int {
	cmp := bytes.Compare(a, b)
	if cfg.reverse {
		return -cmp
	}
	return cmp
}

// keysEqual returns true if all keys compare equal (for -u dedup).
func keysEqual(a, b []byte, cfg config) bool {
	for _, key := range cfg.keys {
		ka := extractKey(a, key, cfg.separator)
		kb := extractKey(b, key, cfg.separator)
		mods := effectiveMods(key.mods, cfg)
		if compareByMods(ka, kb, mods) != 0 {
			return false
		}
	}
	return true
}

// effectiveMods returns the modifiers to use for a key comparison.
// If the key has local modifiers, use those; otherwise inherit global.
func effectiveMods(mods keyMods, cfg config) keyMods {
	if mods.hasAnyMod() {
		return mods
	}
	return keyMods{
		numeric: cfg.numeric,
		reverse: cfg.reverse,
	}
}

// compareByMods compares two key values using the specified modifiers.
func compareByMods(a, b []byte, mods keyMods) int {
	if mods.blanks {
		a = trimLeadingBlanks(a)
		b = trimLeadingBlanks(b)
	}
	if mods.numeric {
		return numericCompare(a, b)
	}
	return bytes.Compare(a, b)
}

// --- Non-key comparison (whole-line mode) ---

// selectCompare returns the less-than function for non-key sorting.
func selectCompare(cfg config) func(a, b []byte) bool {
	var less func(a, b []byte) bool
	if cfg.numeric {
		less = numericLess
	} else {
		less = byteLess
	}
	if cfg.reverse {
		fwd := less
		less = func(a, b []byte) bool { return fwd(b, a) }
	}
	return less
}

// selectEqual returns the equality function for non-key deduplication.
func selectEqual(cfg config) func(a, b []byte) bool {
	if cfg.numeric {
		return func(a, b []byte) bool {
			va := parseLeadingNumber(a)
			vb := parseLeadingNumber(b)
			return va == vb || (math.IsNaN(va) && math.IsNaN(vb))
		}
	}
	return func(a, b []byte) bool {
		return bytes.Equal(a, b)
	}
}

// byteLess compares two lines lexicographically by raw byte values.
func byteLess(a, b []byte) bool {
	return bytes.Compare(a, b) < 0
}

// numericLess compares two lines by leading numeric value.
func numericLess(a, b []byte) bool {
	va := parseLeadingNumber(a)
	vb := parseLeadingNumber(b)
	if va != vb {
		return va < vb
	}
	return bytes.Compare(a, b) < 0
}

// numericCompare compares two byte slices by leading numeric value.
func numericCompare(a, b []byte) int {
	va := parseLeadingNumber(a)
	vb := parseLeadingNumber(b)
	if va < vb {
		return -1
	}
	if va > vb {
		return 1
	}
	return 0
}

// parseLeadingNumber extracts the leading numeric value from a byte slice.
func parseLeadingNumber(line []byte) float64 {
	s := strings.TrimLeft(string(line), " \t")
	if len(s) == 0 {
		return 0
	}
	end := numericEnd(s)
	if end == 0 {
		return 0
	}
	val, err := strconv.ParseFloat(s[:end], 64)
	if err != nil {
		return 0
	}
	return val
}

// numericEnd finds the end index of a leading numeric value in s.
func numericEnd(s string) int {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	start := i
	hasDot := false
	for i < len(s) {
		if s[i] >= '0' && s[i] <= '9' {
			i++
		} else if s[i] == '.' && !hasDot {
			hasDot = true
			i++
		} else {
			break
		}
	}
	if i == start && hasDot {
		return 0
	}
	if i == start && i > 0 {
		return 0 // sign only, no digits
	}
	return i
}

// trimLeadingBlanks strips leading spaces and tabs from b.
func trimLeadingBlanks(b []byte) []byte {
	i := 0
	for i < len(b) && isBlank(b[i]) {
		i++
	}
	return b[i:]
}
