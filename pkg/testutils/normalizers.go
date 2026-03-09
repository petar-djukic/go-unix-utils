// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import "regexp"

// composeSlice applies a slice of NormalizeFunc in order. Returns nil if the
// slice is empty.
func composeSlice(fns []NormalizeFunc) NormalizeFunc {
	if len(fns) == 0 {
		return nil
	}
	return ComposeNormalizers(fns...)
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order. R4.4.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// timestampPatterns matches common strftime-formatted timestamp patterns. R4.2.
var timestampPatterns = []*regexp.Regexp{
	// ISO 8601: 2026-02-19 12:34:56 or 2026-02-19T12:34:56
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`),
	// ctime-style: Feb 19 12:34:56
	regexp.MustCompile(`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}`),
	// Time only: 12:34:56
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
}

// timestampPlaceholder is the fixed string that replaces timestamp patterns.
const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string. R4.2.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	result := data
	for _, re := range timestampPatterns {
		result = re.ReplaceAll(result, []byte(timestampPlaceholder))
	}
	return result
}
