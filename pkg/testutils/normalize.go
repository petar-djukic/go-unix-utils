// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd001-testutils R3.1–R3.4, R4.1–R4.4.

package testutils

import (
	"regexp"
)

// applyNormalizers applies each NormalizeFunc in order to the data. R3.1, R4.1, R4.3.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}

// ComposeNormalizers returns a single NormalizeFunc that applies the given functions
// in order. R4.4.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// timestampPattern matches common strftime-formatted timestamps. R4.2.
var timestampPattern = regexp.MustCompile(
	// Mon DD HH:MM:SS (e.g., "Feb 19 12:34:56")
	`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` +
		`|` +
		// YYYY-MM-DD HH:MM:SS
		`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}` +
		`|` +
		// HH:MM:SS
		`\d{2}:\d{2}:\d{2}`,
)

// timestampPlaceholder is the fixed string that replaces timestamps.
const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string. R4.2.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	return timestampPattern.ReplaceAll(data, []byte(timestampPlaceholder))
}
