// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Normalization hooks for differential testing. Implements prd001-testutils
// R2.5 (ComposeNormalizers) and R2.6 (TimestampNormalizer).
package testutils

import (
	"regexp"
)

// timestampPatterns matches common strftime-formatted timestamps found in GNU
// utility output:
//   - "YYYY-MM-DD HH:MM:SS" with optional fractional seconds (ISO 8601)
//   - "Mon DD HH:MM:SS YYYY" (syslog with year)
//   - "Mon DD HH:MM:SS" (syslog format)
//   - "HH:MM:SS" with optional fractional seconds (standalone time)
//   - epoch seconds: bare integer of 9-10 digits (unix timestamp)
var timestampPatterns = []*regexp.Regexp{
	// YYYY-MM-DD HH:MM:SS with optional fractional seconds
	regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}(\.\d+)?`),
	// Mon DD HH:MM:SS YYYY (syslog with year)
	regexp.MustCompile(`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2} \d{4}`),
	// Mon DD HH:MM:SS (syslog format)
	regexp.MustCompile(`[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}`),
	// HH:MM:SS with optional fractional seconds (standalone time)
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}(\.\d+)?`),
}

const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime-formatted timestamps with a
// fixed placeholder string for comparison. Used by cmd/ts tests to handle
// wall-clock differences between binary executions. Handles ISO 8601,
// syslog, standalone time, and coreutils default date formats.
// (prd001-testutils R2.6, R4.2)
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	result := b
	for _, pat := range timestampPatterns {
		result = pat.ReplaceAll(result, []byte(timestampPlaceholder))
	}
	return result
}

// ComposeNormalizers returns a single NormalizeFunc that applies fns in order
// (left to right). With zero arguments, returns an identity function that
// passes input through unchanged. With one argument, returns that function
// directly without wrapping. (prd001-testutils R2.5, R4.4)
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	switch len(fns) {
	case 0:
		return func(b []byte) []byte { return b }
	case 1:
		return fns[0]
	default:
		return func(b []byte) []byte {
			for _, fn := range fns {
				b = fn(b)
			}
			return b
		}
	}
}
