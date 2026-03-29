// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import "regexp"

// timestampPlaceholder is the fixed replacement string for normalized timestamps.
const timestampPlaceholder = "<TIMESTAMP>"

// timestampPatterns matches common timestamp formats in command output.
// D3: covers ISO 8601 and common Unix timestamp formats.
var timestampPatterns = regexp.MustCompile(
	// ISO 8601: 2026-03-28T12:34:56 or 2026-03-28 12:34:56
	`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?([+-]\d{2}:?\d{2}|Z)?` +
		`|` +
		// Mon-style: Mar 28 12:34:56 (strftime %b %d %H:%M:%S)
		`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}` +
		`|` +
		// Time only: 12:34:56 or 12:34:56.123456
		`\d{2}:\d{2}:\d{2}(\.\d+)?` +
		`|` +
		// Unix epoch seconds: 1711612496 (10-digit)
		`\b\d{10}\b`,
)

// TimestampNormalizer replaces common timestamp patterns in output with a
// fixed placeholder so time-dependent output can be compared deterministically.
//
// R4.2: built-in normalizer for strftime timestamp patterns.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	return timestampPatterns.ReplaceAll(data, []byte(timestampPlaceholder))
}

// ComposeNormalizers chains multiple NormalizeFunc values into a single
// NormalizeFunc that applies each in order.
//
// R4.3, R4.4: composition of normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			if fn != nil {
				data = fn(data)
			}
		}
		return data
	}
}
