// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd001-testutils R2.5 normalization support: ComposeNormalizers
// and TimestampNormalizer.

package testutils

import (
	"regexp"
)

// timestampPattern matches common strftime-formatted timestamps:
//   - "Mon DD HH:MM:SS" (e.g., "Feb 19 12:34:56")
//   - "YYYY-MM-DD HH:MM:SS" (e.g., "2026-02-19 12:34:56")
//   - "HH:MM:SS" (e.g., "12:34:56")
//   - Timestamps with fractional seconds (e.g., "12:34:56.123456")
var timestampPattern = regexp.MustCompile(
	`(?:` +
		`[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}` + // Mon DD HH:MM:SS
		`|` +
		`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}` + // YYYY-MM-DD HH:MM:SS
		`|` +
		`\d{2}:\d{2}:\d{2}` + // HH:MM:SS
		`)` +
		`(?:\.\d+)?`, // optional fractional seconds
)

const timestampPlaceholder = "<TIMESTAMP>"

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string. Used by cmd/ts tests to avoid wall-clock divergence.
//
// R4.2: Built-in TimestampNormalizer.
var TimestampNormalizer NormalizeFunc = func(b []byte) []byte {
	return timestampPattern.ReplaceAll(b, []byte(timestampPlaceholder))
}

// ComposeNormalizers combines multiple NormalizeFunc values into a single
// NormalizeFunc that applies them left to right. Returns nil if no functions
// are provided.
//
// R4.4: Convenience for combining multiple normalizers.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	if len(fns) == 0 {
		return nil
	}
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}
