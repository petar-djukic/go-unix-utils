// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Normalization helpers for differential testing.
// Implements srd001-testutils R4.2, R4.4 (ComposeNormalizers, TimestampNormalizer)
// and task requirements R3 (path normalization), R4 (whitespace normalization).
package testutils

import (
	"bytes"
	"regexp"
)

// timestampPlaceholder is the fixed string that replaces matched timestamps.
const timestampPlaceholder = "TIMESTAMP"

// pathPlaceholder is the fixed string that replaces temporary directory prefixes.
const pathPlaceholder = "TMPDIR"

// Regex patterns for common timestamp formats.
var (
	// ISO 8601: 2026-04-06T12:34:56, 2026-04-06 12:34:56, with optional fractional seconds and timezone.
	isoTimestampRe = regexp.MustCompile(
		`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`,
	)

	// ctime/date-style: Mon Jan  2 15:04:05 2006, Mon Jan  2 15:04:05 MST 2006.
	ctimeRe = regexp.MustCompile(
		`(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun)\s+` +
			`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+` +
			`\d{1,2}\s+\d{2}:\d{2}:\d{2}(?:\s+\w+)?\s+\d{4}`,
	)

	// Syslog-style: Jan 02 15:04:05 (month day time, no year).
	syslogRe = regexp.MustCompile(
		`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`,
	)

	// HH:MM:SS standalone time format.
	timeOnlyRe = regexp.MustCompile(`\d{2}:\d{2}:\d{2}`)

	// Unix epoch seconds with optional fractional part: 1712345678 or 1712345678.123456.
	epochRe = regexp.MustCompile(`\b\d{10}(?:\.\d+)?\b`)

	// Common temp directory prefixes on macOS and Linux.
	tempDirRe = regexp.MustCompile(
		`(?:/private)?/(?:tmp|var/folders)/[^\s:,"']+`,
	)
)

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order (left-to-right). Returns an identity function when
// fns is empty.
// R4.4: ComposeNormalizers(a, b)(input) == b(a(input)).
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// TimestampNormalizer replaces common strftime timestamp patterns with a
// fixed placeholder string. Used by cmd/ts tests.
// R4.2: handles ISO 8601, ctime, syslog, HH:MM:SS, and Unix epoch formats.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	placeholder := []byte(timestampPlaceholder)
	b = isoTimestampRe.ReplaceAll(b, placeholder)
	b = ctimeRe.ReplaceAll(b, placeholder)
	b = syslogRe.ReplaceAll(b, placeholder)
	b = epochRe.ReplaceAll(b, placeholder)
	b = timeOnlyRe.ReplaceAll(b, placeholder)
	return b
}

// PathNormalizer replaces absolute paths containing temporary directory
// prefixes with a fixed placeholder so tests are not sensitive to temp
// dir names.
// R3.5: normalizes /tmp, /var/folders, and /private/tmp prefixes.
var PathNormalizer NormalizeFunc = func(b []byte) []byte {
	return tempDirRe.ReplaceAll(b, []byte(pathPlaceholder))
}

// TrailingWhitespaceNormalizer removes trailing whitespace (spaces and tabs)
// from each line so differential tests pass when the SRD marks trailing
// whitespace differences as acceptable divergence.
// R3.6: strips trailing spaces and tabs per line, preserves newlines.
var TrailingWhitespaceNormalizer NormalizeFunc = func(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		lines[i] = bytes.TrimRight(line, " \t")
	}
	return bytes.Join(lines, []byte("\n"))
}
