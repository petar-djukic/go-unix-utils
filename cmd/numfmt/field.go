// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd071-numfmt R3.1 (--field), R3.2 (--delimiter):
// field selection parsing and field-based line processing.
package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// fieldRange represents a single range in a field specification.
type fieldRange struct {
	lo int // 1-indexed; 0 means from field 1
	hi int // 1-indexed; 0 means to infinity
}

// fieldSet holds parsed field ranges for --field processing. R3.1.
type fieldSet struct {
	ranges []fieldRange
}

// contains returns true if field number n (1-indexed) is selected.
func (f *fieldSet) contains(n int) bool {
	for _, r := range f.ranges {
		lo := r.lo
		if lo == 0 {
			lo = 1
		}
		if n >= lo && (r.hi == 0 || n <= r.hi) {
			return true
		}
	}
	return false
}

// parseFieldSpec parses a field specification like "1", "1-3", "2,5-".
func parseFieldSpec(spec string) (fieldSet, error) {
	var fs fieldSet
	parts := strings.Split(spec, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		r, err := parseSingleRange(p)
		if err != nil {
			return fs, err
		}
		fs.ranges = append(fs.ranges, r)
	}
	if len(fs.ranges) == 0 {
		return fs, fmt.Errorf("invalid field specification: '%s'", spec)
	}
	return fs, nil
}

// parseSingleRange parses one range element (e.g., "1", "2-5", "-3", "4-").
func parseSingleRange(s string) (fieldRange, error) {
	dashIdx := strings.Index(s, "-")
	if dashIdx < 0 {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			return fieldRange{}, fmt.Errorf("invalid field value '%s'", s)
		}
		return fieldRange{lo: n, hi: n}, nil
	}
	return parseDashRange(s, dashIdx)
}

// parseDashRange parses a range containing a dash separator.
func parseDashRange(s string, dashIdx int) (fieldRange, error) {
	var r fieldRange
	if dashIdx > 0 {
		lo, err := strconv.Atoi(s[:dashIdx])
		if err != nil || lo < 1 {
			return r, fmt.Errorf("invalid field range '%s'", s)
		}
		r.lo = lo
	}
	if dashIdx < len(s)-1 {
		hi, err := strconv.Atoi(s[dashIdx+1:])
		if err != nil || hi < 1 {
			return r, fmt.Errorf("invalid field range '%s'", s)
		}
		r.hi = hi
	}
	return r, nil
}

// processLineWithFields converts only selected fields in a line. R3.1.
func processLineWithFields(line string, cfg numfmtConfig, fields *fieldSet, w *bufio.Writer, stderr io.Writer) error {
	if cfg.delimiter != "" {
		return processDelimitedLine(line, cfg, fields, w, stderr)
	}
	return processWhitespaceLine(line, cfg, fields, w, stderr)
}

// processDelimitedLine splits on a specific delimiter and converts selected fields.
func processDelimitedLine(line string, cfg numfmtConfig, fields *fieldSet, w *bufio.Writer, stderr io.Writer) error {
	parts := strings.Split(line, cfg.delimiter)
	var convErr error
	for i, p := range parts {
		if fields.contains(i + 1) {
			result, err := convertValue(p, cfg)
			if err != nil {
				if e := reportInvalid(err, cfg, stderr); e != nil {
					convErr = e
				}
			} else {
				parts[i] = result
			}
		}
	}
	fmt.Fprintln(w, strings.Join(parts, cfg.delimiter))
	return convErr
}

// processWhitespaceLine splits on whitespace runs, preserving separators.
// Converted values are right-aligned to the original token width.
func processWhitespaceLine(line string, cfg numfmtConfig, fields *fieldSet, w *bufio.Writer, stderr io.Writer) error {
	tokens, seps := splitWhitespace(line)
	var convErr error
	for i, tok := range tokens {
		if fields.contains(i + 1) {
			result, err := convertValue(tok, cfg)
			if err != nil {
				if e := reportInvalid(err, cfg, stderr); e != nil {
					convErr = e
				}
			} else {
				tokens[i] = padToOriginalWidth(result, len(tok))
			}
		}
	}
	fmt.Fprintln(w, joinWithSeps(tokens, seps))
	return convErr
}

// padToOriginalWidth right-aligns result to at least origWidth characters.
func padToOriginalWidth(result string, origWidth int) string {
	if len(result) >= origWidth {
		return result
	}
	return strings.Repeat(" ", origWidth-len(result)) + result
}

// splitWhitespace splits a line into tokens and their separating whitespace.
// seps[0] is leading whitespace, seps[i+1] follows tokens[i].
func splitWhitespace(line string) ([]string, []string) {
	var tokens []string
	var seps []string
	i := 0
	start := i
	for i < len(line) && isBlank(line[i]) {
		i++
	}
	seps = append(seps, line[start:i])
	for i < len(line) {
		start = i
		for i < len(line) && !isBlank(line[i]) {
			i++
		}
		tokens = append(tokens, line[start:i])
		start = i
		for i < len(line) && isBlank(line[i]) {
			i++
		}
		seps = append(seps, line[start:i])
	}
	return tokens, seps
}

// isBlank returns true if c is a space or tab.
func isBlank(c byte) bool {
	return c == ' ' || c == '\t'
}

// joinWithSeps reconstructs a line from tokens and their separators.
func joinWithSeps(tokens []string, seps []string) string {
	var b strings.Builder
	b.WriteString(seps[0])
	for i, tok := range tokens {
		b.WriteString(tok)
		if i+1 < len(seps) {
			b.WriteString(seps[i+1])
		}
	}
	return b.String()
}
