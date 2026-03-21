// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd001-testutils R4.1-R4.4: ComposeNormalizers and
// TimestampNormalizer.
package testutils

import (
	"bytes"
	"strings"
	"testing"
)

// TestComposeNormalizers_ZeroArgs verifies R4.4: ComposeNormalizers with
// zero arguments returns a NormalizeFunc that returns input unchanged.
func TestComposeNormalizers_ZeroArgs(t *testing.T) {
	t.Parallel()
	fn := ComposeNormalizers()
	input := []byte("hello world\n")
	result := fn(input)
	if !bytes.Equal(input, result) {
		t.Fatalf("expected %q, got %q", input, result)
	}
}

// TestComposeNormalizers_SingleFunc verifies R4.1: a single function
// is applied correctly.
func TestComposeNormalizers_SingleFunc(t *testing.T) {
	t.Parallel()
	upper := func(b []byte) []byte {
		return []byte(strings.ToUpper(string(b)))
	}
	fn := ComposeNormalizers(upper)
	result := fn([]byte("hello"))
	if string(result) != "HELLO" {
		t.Fatalf("expected %q, got %q", "HELLO", string(result))
	}
}

// TestComposeNormalizers_ChainsInOrder verifies R4.1: multiple functions
// are applied in the order they are given.
func TestComposeNormalizers_ChainsInOrder(t *testing.T) {
	t.Parallel()
	addA := func(b []byte) []byte { return append(b, 'A') }
	addB := func(b []byte) []byte { return append(b, 'B') }
	addC := func(b []byte) []byte { return append(b, 'C') }
	fn := ComposeNormalizers(addA, addB, addC)
	result := fn([]byte("X"))
	expected := "XABC"
	if string(result) != expected {
		t.Fatalf("expected %q, got %q", expected, string(result))
	}
}

// TestComposeNormalizers_EmptyInput verifies R4.4: composed function
// handles empty input without error.
func TestComposeNormalizers_EmptyInput(t *testing.T) {
	t.Parallel()
	upper := func(b []byte) []byte {
		return []byte(strings.ToUpper(string(b)))
	}
	fn := ComposeNormalizers(upper)
	result := fn([]byte{})
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %q", string(result))
	}
}

// TestTimestampNormalizer_ISO8601 verifies R4.2: ISO 8601 timestamps
// are replaced with the placeholder.
func TestTimestampNormalizer_ISO8601(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"date-T-time",
			"event at 2026-03-17T14:30:45 done",
			"event at <TIMESTAMP> done",
		},
		{
			"date-space-time",
			"event at 2026-03-17 14:30:45 done",
			"event at <TIMESTAMP> done",
		},
		{
			"with-subseconds",
			"event at 2026-03-17T14:30:45.123456 done",
			"event at <TIMESTAMP> done",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := TimestampNormalizer([]byte(tc.input))
			if string(result) != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, string(result))
			}
		})
	}
}

// TestTimestampNormalizer_Syslog verifies R4.2: syslog-style timestamps
// (Mon DD HH:MM:SS) are replaced.
func TestTimestampNormalizer_Syslog(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"double-digit-day",
			"Mar 17 12:34:56 event",
			"<TIMESTAMP> event",
		},
		{
			"single-digit-day",
			"Feb  3 09:15:00 event",
			"<TIMESTAMP> event",
		},
		{
			"with-subseconds",
			"Jan 01 00:00:00.999 event",
			"<TIMESTAMP> event",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := TimestampNormalizer([]byte(tc.input))
			if string(result) != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, string(result))
			}
		})
	}
}

// TestTimestampNormalizer_TimeOnly verifies R4.2: bare HH:MM:SS
// timestamps are replaced.
func TestTimestampNormalizer_TimeOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"plain",
			"prefix 14:30:45 suffix",
			"prefix <TIMESTAMP> suffix",
		},
		{
			"with-subseconds",
			"prefix 14:30:45.123 suffix",
			"prefix <TIMESTAMP> suffix",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := TimestampNormalizer([]byte(tc.input))
			if string(result) != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, string(result))
			}
		})
	}
}

// TestTimestampNormalizer_UnixEpoch verifies R4.2: Unix epoch timestamps
// (10+ digits with optional decimal) are replaced.
func TestTimestampNormalizer_UnixEpoch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"integer",
			"ts 1710700245 end",
			"ts <TIMESTAMP> end",
		},
		{
			"with-decimal",
			"ts 1710700245.123456 end",
			"ts <TIMESTAMP> end",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := TimestampNormalizer([]byte(tc.input))
			if string(result) != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, string(result))
			}
		})
	}
}

// TestTimestampNormalizer_PreservesNonTimestamp verifies R4.4:
// non-timestamp content is not altered.
func TestTimestampNormalizer_PreservesNonTimestamp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
	}{
		{"plain-text", "hello world"},
		{"short-numbers", "count is 42"},
		{"file-path", "/usr/local/bin/myapp"},
		{"empty", ""},
		{"newlines-only", "\n\n\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := TimestampNormalizer([]byte(tc.input))
			if string(result) != tc.input {
				t.Fatalf("expected %q unchanged, got %q", tc.input, string(result))
			}
		})
	}
}

// TestTimestampNormalizer_MultipleTimestamps verifies R4.2: multiple
// timestamps in a single input are all replaced.
func TestTimestampNormalizer_MultipleTimestamps(t *testing.T) {
	t.Parallel()
	input := "start 2026-03-17T10:00:00 middle 2026-03-17T11:00:00 end"
	result := TimestampNormalizer([]byte(input))
	expected := "start <TIMESTAMP> middle <TIMESTAMP> end"
	if string(result) != expected {
		t.Fatalf("expected %q, got %q", expected, string(result))
	}
}

// TestTimestampNormalizer_MultilineInput verifies R4.4: timestamps
// are replaced across multiple lines without altering line structure.
func TestTimestampNormalizer_MultilineInput(t *testing.T) {
	t.Parallel()
	input := "Mar 17 12:34:56 line1\nMar 17 12:34:57 line2\n"
	result := TimestampNormalizer([]byte(input))
	expected := "<TIMESTAMP> line1\n<TIMESTAMP> line2\n"
	if string(result) != expected {
		t.Fatalf("expected %q, got %q", expected, string(result))
	}
}
