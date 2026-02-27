// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import "regexp"

// timestampPattern matches strftime timestamps in the default ts output format
// "%b %d %H:%M:%S" (e.g., "Feb 19 12:34:56"). The month abbreviations are the
// three-letter English names used by strftime %b (prd001 R4.2).
var timestampPattern = regexp.MustCompile(
	`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`,
)

const timestampPlaceholder = "TIMESTAMP"

// TimestampNormalizer is a Normalize function that replaces common strftime
// timestamp patterns (e.g., "Feb 19 12:34:56") with the fixed placeholder
// "TIMESTAMP". Applying this normalizer to both binaries' output before
// comparison allows ts differential tests to pass despite wall-clock differences
// between the reference and Go binary invocations (prd001 R4.2, AC3).
func TimestampNormalizer(in []byte) []byte {
	return timestampPattern.ReplaceAll(in, []byte(timestampPlaceholder))
}

// Compose returns a single Normalize function that applies each element of
// normalizers in order. nil elements are skipped. When normalizers is nil or
// empty, the returned function is a no-op that returns the input unchanged
// (prd001 R4.3).
func Compose(normalizers ...Normalize) Normalize {
	return func(in []byte) []byte {
		for _, n := range normalizers {
			if n != nil {
				in = n(in)
			}
		}
		return in
	}
}
