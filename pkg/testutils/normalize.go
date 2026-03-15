// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R3.1-R3.4: ComposeNormalizers and TimestampNormalizer.

package testutils

import "regexp"

// timestampPlaceholder is the fixed string that replaces timestamp patterns.
// R3.3: deterministic placeholder for non-deterministic timestamps.
const timestampPlaceholder = "<TIMESTAMP>"

// timestampPatterns matches common timestamp formats found in utility output:
//   - ISO-8601:  2026-03-14 12:34:56 or 2026-03-14T12:34:56 with optional fractional seconds
//   - ctime/ls:  Mar 14 12:34:56 or Mar 14  2026
//   - Unix epoch with decimals: 1710412496.123456
//   - HH:MM:SS with optional fractional seconds
//
// R3.4: covers ISO 8601, floating-point epoch seconds, and date-time strings.
var timestampPatterns = regexp.MustCompile(
	`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?` + // ISO-8601
		`|[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` + // ctime: Mar 14 12:34:56
		`|[A-Z][a-z]{2}\s+\d{1,2}\s+\d{4}` + // ls year: Mar 14  2026
		`|\d{10,}\.\d+` + // Unix epoch with decimals
		`|\d{2}:\d{2}:\d{2}(?:\.\d+)?`, // HH:MM:SS with optional fractional
)

// TimestampNormalizer replaces common timestamp patterns with a deterministic
// placeholder so that differential tests pass despite wall-clock differences.
//
// R3.3: Built-in normalizer for strftime timestamp patterns.
// R3.4: Covers ISO 8601, floating-point epoch seconds, and date-time strings.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return timestampPatterns.ReplaceAll(b, []byte(timestampPlaceholder))
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in left-to-right order: the output of fns[i] becomes the input of
// fns[i+1].
//
// R3.1: variadic composition of NormalizeFunc values.
// R3.2: zero functions returns input unchanged; single function returns that
// function directly; nil functions in the list are skipped.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	// Filter out nil functions.
	valid := make([]NormalizeFunc, 0, len(fns))
	for _, fn := range fns {
		if fn != nil {
			valid = append(valid, fn)
		}
	}

	switch len(valid) {
	case 0:
		// R3.2: zero functions returns input unchanged.
		return func(b []byte) []byte { return b }
	case 1:
		// R3.2: single function returns that function directly.
		return valid[0]
	default:
		return func(b []byte) []byte {
			for _, fn := range valid {
				b = fn(b)
			}
			return b
		}
	}
}
