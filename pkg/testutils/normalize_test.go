// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd001-testutils R3.1–R3.4, R4.1–R4.4 normalization infrastructure.

package testutils

import (
	"bytes"
	"testing"
)

func TestApplyNormalizers_NilSlice(t *testing.T) {
	t.Parallel()

	input := []byte("hello world")
	got := applyNormalizers(input, nil)
	if !bytes.Equal(got, input) {
		t.Errorf("applyNormalizers(input, nil) = %q, want %q", got, input)
	}
}

func TestApplyNormalizers_EmptySlice(t *testing.T) {
	t.Parallel()

	input := []byte("hello world")
	got := applyNormalizers(input, []NormalizeFunc{})
	if !bytes.Equal(got, input) {
		t.Errorf("applyNormalizers(input, []) = %q, want %q", got, input)
	}
}

func TestApplyNormalizers_SingleFunc(t *testing.T) {
	t.Parallel()

	upper := func(data []byte) []byte {
		result := make([]byte, len(data))
		for i, b := range data {
			if b >= 'a' && b <= 'z' {
				result[i] = b - 32
			} else {
				result[i] = b
			}
		}
		return result
	}

	input := []byte("hello")
	got := applyNormalizers(input, []NormalizeFunc{upper})
	want := []byte("HELLO")
	if !bytes.Equal(got, want) {
		t.Errorf("applyNormalizers with upper = %q, want %q", got, want)
	}
}

func TestApplyNormalizers_OrderMatters(t *testing.T) {
	t.Parallel()

	// First: replace "a" with "b", Second: replace "b" with "c"
	replaceAB := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("a"), []byte("b"))
	}
	replaceBC := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("b"), []byte("c"))
	}

	input := []byte("a")
	got := applyNormalizers(input, []NormalizeFunc{replaceAB, replaceBC})
	// a -> b -> c
	want := []byte("c")
	if !bytes.Equal(got, want) {
		t.Errorf("applyNormalizers order = %q, want %q", got, want)
	}
}

func TestComposeNormalizers_Empty(t *testing.T) {
	t.Parallel()

	composed := ComposeNormalizers()
	input := []byte("hello")
	got := composed(input)
	if !bytes.Equal(got, input) {
		t.Errorf("ComposeNormalizers()(%q) = %q, want %q", input, got, input)
	}
}

func TestComposeNormalizers_Single(t *testing.T) {
	t.Parallel()

	trimSpace := func(data []byte) []byte {
		return bytes.TrimSpace(data)
	}

	composed := ComposeNormalizers(trimSpace)
	input := []byte("  hello  ")
	got := composed(input)
	want := []byte("hello")
	if !bytes.Equal(got, want) {
		t.Errorf("ComposeNormalizers(trim)(%q) = %q, want %q", input, got, want)
	}
}

func TestComposeNormalizers_Multiple(t *testing.T) {
	t.Parallel()

	// Replace "foo" with "bar", then trim trailing newline
	replaceFoo := func(data []byte) []byte {
		return bytes.ReplaceAll(data, []byte("foo"), []byte("bar"))
	}
	trimNewline := func(data []byte) []byte {
		return bytes.TrimRight(data, "\n")
	}

	composed := ComposeNormalizers(replaceFoo, trimNewline)
	input := []byte("foo baz\n")
	got := composed(input)
	want := []byte("bar baz")
	if !bytes.Equal(got, want) {
		t.Errorf("ComposeNormalizers(replace, trim)(%q) = %q, want %q", input, got, want)
	}
}

func TestComposeNormalizers_Composable(t *testing.T) {
	t.Parallel()

	// R4.4: ComposeNormalizers result is itself a NormalizeFunc, composable with others
	upper := func(data []byte) []byte {
		result := make([]byte, len(data))
		for i, b := range data {
			if b >= 'a' && b <= 'z' {
				result[i] = b - 32
			} else {
				result[i] = b
			}
		}
		return result
	}

	inner := ComposeNormalizers(TimestampNormalizer)
	outer := ComposeNormalizers(inner, upper)

	input := []byte("event at 12:34:56")
	got := outer(input)
	want := []byte("EVENT AT " + timestampPlaceholder)
	// TimestampNormalizer replaces "12:34:56" with "<TIMESTAMP>", then upper uppercases
	// but <TIMESTAMP> has no lowercase letters, only "EVENT AT " is affected
	if !bytes.Equal(got, want) {
		t.Errorf("nested ComposeNormalizers = %q, want %q", got, want)
	}
}

func TestTimestampNormalizer_Patterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "HH:MM:SS",
			input: "event at 12:34:56 done",
			want:  "event at <TIMESTAMP> done",
		},
		{
			name:  "YYYY-MM-DD_HH:MM:SS",
			input: "2026-03-09 14:22:01 log entry",
			want:  "<TIMESTAMP> log entry",
		},
		{
			name:  "Mon_DD_HH:MM:SS",
			input: "Feb 19 12:34:56 syslog",
			want:  "<TIMESTAMP> syslog",
		},
		{
			name:  "no_timestamp",
			input: "no timestamps here",
			want:  "no timestamps here",
		},
		{
			name:  "multiple_timestamps",
			input: "start 01:02:03 end 04:05:06",
			want:  "start <TIMESTAMP> end <TIMESTAMP>",
		},
		{
			name:  "empty_input",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := TimestampNormalizer([]byte(tc.input))
			if string(got) != tc.want {
				t.Errorf("TimestampNormalizer(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestTimestampNormalizer_IsNormalizeFunc(t *testing.T) {
	t.Parallel()

	// R4.2: TimestampNormalizer must satisfy NormalizeFunc signature
	var fn NormalizeFunc = TimestampNormalizer
	got := fn([]byte("12:00:00"))
	want := []byte(timestampPlaceholder)
	if !bytes.Equal(got, want) {
		t.Errorf("TimestampNormalizer as NormalizeFunc = %q, want %q", got, want)
	}
}

func TestTimestampNormalizer_ComposableWithComposeNormalizers(t *testing.T) {
	t.Parallel()

	// R4.4: TimestampNormalizer composable via ComposeNormalizers
	addPrefix := func(data []byte) []byte {
		return append([]byte("LOG: "), data...)
	}

	composed := ComposeNormalizers(TimestampNormalizer, addPrefix)
	input := []byte("12:34:56 event")
	got := composed(input)
	want := []byte("LOG: <TIMESTAMP> event")
	if !bytes.Equal(got, want) {
		t.Errorf("composed = %q, want %q", got, want)
	}
}
