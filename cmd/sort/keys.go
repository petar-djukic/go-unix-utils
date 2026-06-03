// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"strconv"
	"strings"
)

type keyDef struct {
	startField  int
	startChar   int
	endField    int
	endChar     int
	mode        sortMode
	reverse     bool
	blanks      bool
	dictOrder   bool
	foldCase    bool
	ignorePrint bool
	hasOpts     bool
}

func parseKeyDef(s string) (keyDef, error) {
	parts := strings.SplitN(s, ",", 2)
	f, c, opts, err := parseKeyPart(parts[0])
	if err != nil {
		return keyDef{}, fmt.Errorf("invalid key: %s", s)
	}
	k := keyDef{startField: f, startChar: c}
	if opts != "" {
		k.hasOpts = true
		if err := applyKeyOpts(&k, opts); err != nil {
			return keyDef{}, err
		}
	}
	if len(parts) > 1 {
		f, c, opts, err = parseKeyPart(parts[1])
		if err != nil {
			return keyDef{}, fmt.Errorf("invalid key: %s", s)
		}
		k.endField = f
		k.endChar = c
		if opts != "" {
			k.hasOpts = true
			if err := applyKeyOpts(&k, opts); err != nil {
				return keyDef{}, err
			}
		}
	}
	if k.startField < 1 {
		return keyDef{}, fmt.Errorf("field number is zero: invalid count at start of %q", s)
	}
	if k.startChar == 0 {
		k.startChar = 1
	}
	return k, nil
}

func parseKeyPart(s string) (int, int, string, error) {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0, 0, "", fmt.Errorf("invalid number at field start")
	}
	field, _ := strconv.Atoi(s[:i])
	char := 0
	if i < len(s) && s[i] == '.' {
		i++
		j := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i > j {
			char, _ = strconv.Atoi(s[j:i])
		}
	}
	return field, char, s[i:], nil
}

func applyKeyOpts(k *keyDef, opts string) error {
	for _, c := range opts {
		switch c {
		case 'b':
			k.blanks = true
		case 'd':
			k.dictOrder = true
		case 'f':
			k.foldCase = true
		case 'i':
			k.ignorePrint = true
		case 'n':
			k.mode = sortNumeric
		case 'h':
			k.mode = sortHumanNumeric
		case 'M':
			k.mode = sortMonth
		case 'V':
			k.mode = sortVersion
		case 'r':
			k.reverse = true
		default:
			return fmt.Errorf("invalid option for key: %c", c)
		}
	}
	return nil
}

func resolveKeyOpts(k keyDef, opts options) keyDef {
	if k.hasOpts {
		return k
	}
	k.mode = opts.mode
	k.reverse = opts.reverse
	k.blanks = opts.ignoreBlanks
	return k
}

func buildKeysCmp(opts options) func(string, string) int {
	resolved := make([]keyDef, len(opts.keys))
	for i, k := range opts.keys {
		resolved[i] = resolveKeyOpts(k, opts)
	}
	sep := opts.fieldSeparator
	return func(a, b string) int {
		for _, k := range resolved {
			ka := extractKey(a, k, sep)
			kb := extractKey(b, k, sep)
			ka = filterKey(ka, k)
			kb = filterKey(kb, k)
			r := modeCompare(ka, kb, k.mode)
			if k.reverse {
				r = -r
			}
			if r != 0 {
				return r
			}
		}
		return 0
	}
}

func extractKey(line string, k keyDef, sep string) string {
	start := fieldPos(line, k.startField, sep)
	if k.blanks {
		start = skipBlanks(line, start)
	}
	if k.startChar > 1 {
		start += k.startChar - 1
	}
	if start >= len(line) {
		return ""
	}
	if k.endField == 0 {
		return line[start:]
	}
	end := endBound(line, k, sep)
	if end <= start {
		return ""
	}
	return line[start:end]
}

func endBound(line string, k keyDef, sep string) int {
	if k.endChar == 0 {
		return fieldEnd(line, k.endField, sep)
	}
	pos := fieldPos(line, k.endField, sep)
	if k.blanks {
		pos = skipBlanks(line, pos)
	}
	pos += k.endChar
	if pos > len(line) {
		return len(line)
	}
	return pos
}

func fieldPos(line string, field int, sep string) int {
	if sep != "" {
		return fieldPosSep(line, field, sep)
	}
	return fieldPosDefault(line, field)
}

func fieldPosDefault(line string, field int) int {
	pos := 0
	for f := 1; f < field; f++ {
		for pos < len(line) && isBlank(line[pos]) {
			pos++
		}
		for pos < len(line) && !isBlank(line[pos]) {
			pos++
		}
	}
	return pos
}

func fieldPosSep(line string, field int, sep string) int {
	pos := 0
	for f := 1; f < field; f++ {
		idx := strings.Index(line[pos:], sep)
		if idx < 0 {
			return len(line)
		}
		pos += idx + 1
	}
	return pos
}

func fieldEnd(line string, field int, sep string) int {
	if sep != "" {
		return fieldEndSep(line, field, sep)
	}
	return fieldEndDefault(line, field)
}

func fieldEndDefault(line string, field int) int {
	pos := fieldPosDefault(line, field)
	for pos < len(line) && isBlank(line[pos]) {
		pos++
	}
	for pos < len(line) && !isBlank(line[pos]) {
		pos++
	}
	return pos
}

func fieldEndSep(line string, field int, sep string) int {
	pos := fieldPosSep(line, field, sep)
	idx := strings.Index(line[pos:], sep)
	if idx < 0 {
		return len(line)
	}
	return pos + idx
}

func skipBlanks(line string, pos int) int {
	for pos < len(line) && isBlank(line[pos]) {
		pos++
	}
	return pos
}

func isBlank(c byte) bool {
	return c == ' ' || c == '\t'
}

func filterKey(s string, k keyDef) string {
	if !k.dictOrder && !k.foldCase && !k.ignorePrint {
		return s
	}
	buf := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if k.ignorePrint && (c < 32 || c > 126) {
			continue
		}
		if k.dictOrder && !isBlank(c) && !isAlnum(c) {
			continue
		}
		if k.foldCase && c >= 'a' && c <= 'z' {
			c -= 32
		}
		buf = append(buf, c)
	}
	return string(buf)
}

func isAlnum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
