// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R4.1–R4.4: normalization hooks.

package testutils

import (
	"regexp"
)

// timestampPlaceholder replaces matched timestamp patterns.
const timestampPlaceholder = "<TIMESTAMP>"

// timestampPatterns matches common strftime-formatted timestamps.
// Covers patterns like "Mar 19 12:34:56", "2026-03-19 12:34:56",
// "12:34:56", and ISO-like "2026-03-19T12:34:56".
var timestampPatterns = []*regexp.Regexp{
	// "Mar 19 12:34:56" — syslog/ts default format
	regexp.MustCompile(`[A-Z][a-z]{2}\s+\d{1,2}\s\d{2}:\d{2}:\d{2}`),
	// "2026-03-19 12:34:56" or "2026-03-19T12:34:56" — ISO-like
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`),
	// "HH:MM:SS" standalone — elapsed/incremental timestamps
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
}

// ComposeNormalizers chains multiple NormalizeFunc values into a single
// NormalizeFunc that applies them in order.
//
// R4.4: convenience for cmd/ test files combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// TimestampNormalizer replaces common timestamp patterns in output bytes
// with a fixed placeholder so that time-dependent tests can pass.
//
// R4.2: built-in normalizer for strftime timestamp patterns.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	result := b
	for _, pat := range timestampPatterns {
		result = pat.ReplaceAll(result, []byte(timestampPlaceholder))
	}
	return result
}
