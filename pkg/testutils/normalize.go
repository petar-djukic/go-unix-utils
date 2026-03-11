// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd001-testutils R4.2, R4.4
package testutils

import "regexp"

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
