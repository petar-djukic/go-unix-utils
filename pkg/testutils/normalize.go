// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import "regexp"

// timestampPlaceholder replaces matched timestamp patterns during normalization.
const timestampPlaceholder = "<TIMESTAMP>"

// NormalizeFunc transforms raw output bytes before comparison.
// R2.2: type alias so callers can use func([]byte) []byte interchangeably.
type NormalizeFunc = func([]byte) []byte

// timestampPatterns matches common strftime-formatted timestamps.
// R2.4: used by TimestampNormalizer.
var timestampPatterns = []*regexp.Regexp{
	// ISO 8601: 2024-02-19 12:34:56 or 2024-02-19T12:34:56 with optional fractional seconds
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?`),
	// ctime-style: Feb 19 12:34:56
	regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
	// Unix epoch seconds with fractional part
	regexp.MustCompile(`\d{10,}\.\d+`),
	// Time only: 12:34:56 with optional fractional seconds
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}(\.\d+)?`),
}

// TimestampNormalizer replaces common timestamp patterns with a fixed placeholder.
// R2.4: used by cmd/ts tests to normalize wall-clock differences.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	for _, re := range timestampPatterns {
		b = re.ReplaceAll(b, []byte(timestampPlaceholder))
	}
	return b
}

// ComposeNormalizers returns a single NormalizeFunc that applies fns in order.
// R2.3: convenience for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}
