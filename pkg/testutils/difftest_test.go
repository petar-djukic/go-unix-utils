// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for difftest.go: TimestampNormalizer, ComposeNormalizers, applyNormalizers.
// Implements: prd001-testutils (R4.2, R4.3, R4.4)
package testutils

import (
	"bytes"
	"strings"
	"testing"
)

func TestTimestampNormalizer(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "month-day-time pattern",
			input: "Feb 19 12:34:56",
			want:  "<TIMESTAMP>",
		},
		{
			name:  "ISO datetime pattern",
			input: "2026-02-19 12:34:56",
			want:  "<TIMESTAMP>",
		},
		{
			name:  "time-only pattern",
			input: "12:34:56",
			want:  "<TIMESTAMP>",
		},
		{
			name:  "month-day-time embedded in line",
			input: "prefix Feb 19 12:34:56 suffix",
			want:  "prefix <TIMESTAMP> suffix",
		},
		{
			name:  "ISO datetime embedded in line",
			input: "log: 2026-02-19 12:34:56 event",
			want:  "log: <TIMESTAMP> event",
		},
		{
			name:  "multiple timestamps in one line",
			input: "start 09:00:00 end 17:30:00",
			want:  "start <TIMESTAMP> end <TIMESTAMP>",
		},
		{
			name:  "no timestamps unchanged",
			input: "no timestamps here",
			want:  "no timestamps here",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := string(TimestampNormalizer([]byte(tc.input)))
			if got != tc.want {
				t.Fatalf("TimestampNormalizer(%q)\n  got:  %q\n  want: %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestComposeNormalizers(t *testing.T) {
	t.Run("chains functions in order", func(t *testing.T) {
		upper := func(b []byte) []byte {
			return []byte(strings.ToUpper(string(b)))
		}
		addSuffix := func(b []byte) []byte {
			return append(b, []byte("_DONE")...)
		}

		composed := ComposeNormalizers(upper, addSuffix)
		got := string(composed([]byte("hello")))
		want := "HELLO_DONE"
		if got != want {
			t.Fatalf("ComposeNormalizers result: got %q, want %q", got, want)
		}
	})

	t.Run("single function", func(t *testing.T) {
		double := func(b []byte) []byte {
			return append(b, b...)
		}
		composed := ComposeNormalizers(double)
		got := string(composed([]byte("ab")))
		want := "abab"
		if got != want {
			t.Fatalf("ComposeNormalizers single: got %q, want %q", got, want)
		}
	})

	t.Run("no functions returns input unchanged", func(t *testing.T) {
		composed := ComposeNormalizers()
		input := []byte("unchanged")
		got := composed(input)
		if !bytes.Equal(got, input) {
			t.Fatalf("ComposeNormalizers empty: got %q, want %q", got, input)
		}
	})
}

func TestApplyNormalizers(t *testing.T) {
	t.Run("nil normalizers returns input unchanged", func(t *testing.T) {
		input := []byte("hello world")
		got := applyNormalizers(input, nil)
		if !bytes.Equal(got, input) {
			t.Fatalf("expected unchanged input, got %q", got)
		}
	})

	t.Run("empty normalizers returns input unchanged", func(t *testing.T) {
		input := []byte("hello world")
		got := applyNormalizers(input, []NormalizeFunc{})
		if !bytes.Equal(got, input) {
			t.Fatalf("expected unchanged input, got %q", got)
		}
	})

	t.Run("applies normalizers in order", func(t *testing.T) {
		// First: replace "a" with "b", Second: replace "b" with "c"
		replaceAB := func(b []byte) []byte {
			return bytes.ReplaceAll(b, []byte("a"), []byte("b"))
		}
		replaceBC := func(b []byte) []byte {
			return bytes.ReplaceAll(b, []byte("b"), []byte("c"))
		}

		got := string(applyNormalizers([]byte("abc"), []NormalizeFunc{replaceAB, replaceBC}))
		// "abc" -> replaceAB -> "bbc" -> replaceBC -> "ccc"
		want := "ccc"
		if got != want {
			t.Fatalf("applyNormalizers chain: got %q, want %q", got, want)
		}
	})
}
