// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import "regexp"

// timestampPlaceholder is the fixed replacement string for normalized timestamps.
// R5.1: fixed placeholder prevents false divergences from time-varying output.
const timestampPlaceholder = "<TIMESTAMP>"

// Timestamp patterns are ordered from most specific to least specific so that
// longer matches are consumed before shorter alternatives can partially match.
//
// R5.2: covers ISO 8601, Unix epoch seconds, and common GNU coreutils formats.
var timestampPatterns = regexp.MustCompile(
	// ISO 8601: 2026-03-28T12:34:56, 2026-03-28T12:34:56.123, 2026-03-28T12:34:56+05:30, 2026-03-28T12:34:56Z
	`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?([+-]\d{2}:?\d{2}|Z)?` +
		`|` +
		// Full coreutils date: Mon Jan  2 15:04:05 2006
		`[A-Z][a-z]{2} [A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2} \d{4}` +
		`|` +
		// Month day time with seconds: Jan  2 15:04:05 (syslog, ls old files)
		`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}` +
		`|` +
		// Month day time without seconds: Jan  2 15:04 (ls recent files)
		`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}` +
		`|` +
		// Time only: 12:34:56 or 12:34:56.123456 (ts %H:%M:%S)
		`\d{2}:\d{2}:\d{2}(\.\d+)?` +
		`|` +
		// Unix epoch seconds: 1711612496 (10-digit integer boundary)
		`\b\d{10}\b`,
)

// TimestampNormalizer replaces common timestamp patterns in output with a
// fixed placeholder so time-dependent output can be compared deterministically.
//
// R5.1: NormalizeFunc that replaces timestamp patterns with <TIMESTAMP>.
// R5.2: recognizes ISO 8601, Unix epoch, and common coreutils date formats.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	return timestampPatterns.ReplaceAll(data, []byte(timestampPlaceholder))
}
