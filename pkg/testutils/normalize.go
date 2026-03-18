// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package testutils provides the differential testing harness for go-unix-utils.
// Implements prd001-testutils R1.1–R1.4, R2.1–R2.6, R3.1–R3.6, R4.1–R4.4, R5.1–R5.2.
package testutils

import "regexp"

// NormalizeFunc transforms raw output bytes before comparison.
// Implements prd001-testutils R1.4.
type NormalizeFunc = func([]byte) []byte

// ComposeNormalizers returns a single NormalizeFunc that applies the given
// functions in order. Implements prd001-testutils R4.4.
func ComposeNormalizers(fns ...NormalizeFunc) NormalizeFunc {
	return func(data []byte) []byte {
		for _, fn := range fns {
			data = fn(data)
		}
		return data
	}
}

// timestampPlaceholder is the fixed string that replaces timestamps.
const timestampPlaceholder = "<TIMESTAMP>"

// timestampPatterns matches common strftime timestamp formats, ordered from
// most specific to least specific to avoid partial matches.
var timestampPatterns = regexp.MustCompile(
	`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(\.\d+)?` + // ISO 8601 / YYYY-MM-DD HH:MM:SS
		`|[A-Z][a-z]{2} [ \d]\d \d{2}:\d{2}:\d{2}(\.\d+)?` + // Syslog: Mon DD HH:MM:SS
		`|\d{2}:\d{2}:\d{2}(\.\d+)?` + // HH:MM:SS with optional subseconds
		`|\d{10,}(\.\d+)?`) // Unix epoch with optional decimal

// TimestampNormalizer replaces common strftime timestamp patterns with a
// fixed placeholder so that differential tests pass despite wall-clock
// differences. Implements prd001-testutils R4.2.
var TimestampNormalizer NormalizeFunc = func(data []byte) []byte {
	return timestampPatterns.ReplaceAll(data, []byte(timestampPlaceholder))
}

// applyNormalizers applies a slice of NormalizeFunc to data in order.
// Returns data unchanged when fns is nil or empty.
func applyNormalizers(data []byte, fns []NormalizeFunc) []byte {
	for _, fn := range fns {
		data = fn(data)
	}
	return data
}
