// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides a differential testing harness for verifying Go
// utility implementations against GNU reference binaries.
//
// Implements: prd001-testutils (R1, R2, R3, R4, R5)
package testutils

import "regexp"

// NormalizeFunc transforms raw output bytes before comparison. It is applied to
// both the Go binary and the reference binary outputs so that non-deterministic
// fields (timestamps, PIDs, etc.) do not cause spurious divergence.
//
// Per prd001-testutils R1.4: when nil, no normalization is applied.
type NormalizeFunc func([]byte) []byte

// ComposeNormalizers returns a single NormalizeFunc that applies each function
// in the slice in order. When the slice is nil or empty, the returned function
// passes data through unchanged.
//
// Per prd001-testutils R4.3.
func ComposeNormalizers(fns []NormalizeFunc) NormalizeFunc {
	if len(fns) == 0 {
		return func(b []byte) []byte { return b }
	}
	return func(b []byte) []byte {
		for _, fn := range fns {
			b = fn(b)
		}
		return b
	}
}

// timestampPlaceholder is the fixed string that replaces matched timestamps.
const timestampPlaceholder = "TIMESTAMP"

// timestampPatterns matches common strftime-formatted timestamps:
//   - Syslog/default ts format: "Feb 19 12:34:56" (%b %d %H:%M:%S)
//   - ISO-8601 datetime prefix: "2024-01-05T14:30:00" or with fractional seconds
//   - HH:MM:SS standalone elapsed timestamps (for -s and -i modes)
//   - Seconds with microsecond suffix: "32.001234" (%.S)
//   - Unix epoch with microsecond suffix: "1708358732.001234" (%.s)
var timestampPatterns = []*regexp.Regexp{
	// Syslog format: "Jan  5 14:30:00" or "Feb 19 12:34:56"
	regexp.MustCompile(`[A-Z][a-z]{2}\s{1,2}\d{1,2}\s\d{2}:\d{2}:\d{2}`),
	// ISO-8601: "2024-01-05T14:30:00" with optional fractional seconds and timezone
	regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?`),
	// HH:MM:SS (elapsed time format for -s and -i modes)
	regexp.MustCompile(`\d{2}:\d{2}:\d{2}`),
	// Seconds with microsecond suffix: "32.001234" or epoch "1708358732.001234" (%.S, %.s)
	regexp.MustCompile(`\d+\.\d{6}`),
}

// TimestampNormalizer replaces common strftime timestamp patterns with a fixed
// placeholder string. This normalizer is used by cmd/ts tests so that wall-clock
// differences between the Go binary and the reference binary do not cause test
// failures.
//
// Per prd001-testutils R4.2 and AC3.
func TimestampNormalizer(b []byte) []byte {
	result := b
	for _, pat := range timestampPatterns {
		result = pat.ReplaceAll(result, []byte(timestampPlaceholder))
	}
	return result
}
