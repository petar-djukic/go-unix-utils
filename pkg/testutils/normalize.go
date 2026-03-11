// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd001-testutils R3.3-R3.6, R4.2, R4.4
package testutils

import (
	"bytes"
	"regexp"
	"sort"
)

// timestampPattern matches common timestamp formats produced by ts and related
// utilities. It covers the ts default format (%b %e %H:%M:%S) and ISO 8601.
//
//   - "Feb 19 12:34:56" (ts default: %b %e %H:%M:%S)
//   - "Feb  1 12:34:56" (space-padded single-digit day via %e)
//   - "2026-02-19T12:34:56" / "2026-02-19 12:34:56" (ISO 8601 variants)
var timestampPattern = regexp.MustCompile(
	`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` +
		`|\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`,
)

// timestampPlaceholder is the fixed string substituted for every matched timestamp.
const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common timestamp patterns in output with the
// fixed placeholder "<TIMESTAMP>" so that differential tests for time-stamping
// utilities (e.g., ts) pass despite wall-clock differences.
//
// R4.2: Built-in normalizer for strftime-style timestamps.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return timestampPattern.ReplaceAll(b, []byte(timestampPlaceholder))
}

// ComposeNormalizers returns a single NormalizeFunc that applies fns in order,
// passing the output of each function as the input to the next. When called
// with zero arguments it returns an identity function that returns its input
// unchanged.
//
// R4.4: Convenience composer for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// TrimTrailingWhitespaceNormalizer removes trailing spaces and tabs from every
// line in the output before comparison. Use this when the Go binary and the
// reference binary produce identical content but differ in trailing whitespace
// on individual lines (e.g., column-aligned table utilities that pad with spaces).
//
// R3.3: Built-in normalizer for trailing whitespace differences.
var TrimTrailingWhitespaceNormalizer NormalizeFunc = func(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		lines[i] = bytes.TrimRight(line, " \t")
	}
	return bytes.Join(lines, []byte("\n"))
}

// ansiEscapePattern matches ANSI terminal escape sequences including SGR (color
// and text attribute), cursor movement, and erase sequences. Covers the full
// CSI parameter range used by ls --color and similar utilities.
var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripAnsiCodesNormalizer removes ANSI terminal escape sequences from output
// before comparison. Use this for utilities that emit color or formatting codes
// (e.g., ls --color=always) when the reference binary and Go binary may produce
// different or unexpected escape sequences.
//
// R3.4: Built-in normalizer for ANSI escape sequence differences.
var StripAnsiCodesNormalizer NormalizeFunc = func(b []byte) []byte {
	return ansiEscapePattern.ReplaceAll(b, nil)
}

// SortLinesNormalizer sorts the output lines in ascending byte order before
// comparison. Use this for utilities whose output set is deterministic but whose
// ordering is not guaranteed (e.g., ls in some modes, find, du across inodes).
// A single trailing newline is preserved when present.
//
// R3.5: Built-in normalizer for order-independent line comparison.
var SortLinesNormalizer NormalizeFunc = func(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	// Preserve a trailing newline so the result matches the original structure.
	trailingNewline := b[len(b)-1] == '\n'
	content := b
	if trailingNewline {
		content = b[:len(b)-1]
	}
	lines := bytes.Split(content, []byte("\n"))
	sort.Slice(lines, func(i, j int) bool {
		return bytes.Compare(lines[i], lines[j]) < 0
	})
	result := bytes.Join(lines, []byte("\n"))
	if trailingNewline {
		result = append(result, '\n')
	}
	return result
}

// NormalizeNewlinesNormalizer converts CRLF (\r\n) line endings to LF (\n)
// before comparison. Use this when a utility or platform produces Windows-style
// line endings and the reference binary produces Unix-style line endings.
//
// R3.6: Built-in normalizer for CRLF line ending differences.
var NormalizeNewlinesNormalizer NormalizeFunc = func(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}
