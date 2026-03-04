// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for TimestampNormalizer and ComposeNormalizers normalization functions
// (prd001-testutils R4.2, R4.3, R4.4).
package testutils_test

import (
	"bytes"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestTimestampNormalizer verifies R4.2: TimestampNormalizer replaces common
// strftime timestamp patterns with a fixed placeholder string.
func TestTimestampNormalizer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "syslog-format",
			input: "Feb 19 12:34:56 some event happened",
			want:  "<TIMESTAMP> some event happened",
		},
		{
			name:  "iso-format",
			input: "2024-02-19 12:34:56 some event happened",
			want:  "<TIMESTAMP> some event happened",
		},
		{
			name:  "time-only-format",
			input: "12:34:56 some event happened",
			want:  "<TIMESTAMP> some event happened",
		},
		{
			name:  "no-timestamp",
			input: "plain text with no timestamps",
			want:  "plain text with no timestamps",
		},
		{
			name:  "empty-input",
			input: "",
			want:  "",
		},
		{
			name:  "multiple-timestamps",
			input: "Feb 01 09:00:00 start\n2024-12-31 23:59:59 end\n",
			want:  "<TIMESTAMP> start\n<TIMESTAMP> end\n",
		},
		{
			name:  "syslog-single-digit-day",
			input: "Mar  3 08:15:42 event",
			want:  "<TIMESTAMP> event",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := testutils.TimestampNormalizer([]byte(tc.input))
			if !bytes.Equal(got, []byte(tc.want)) {
				t.Fatalf("TimestampNormalizer(%q)\ngot:  %q\nwant: %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestComposeNormalizers verifies R4.3 and R4.4: ComposeNormalizers returns a
// single NormalizeFunc that applies the given functions in sequence.
func TestComposeNormalizers(t *testing.T) {
	t.Parallel()

	// Two distinct normalizers with observable ordering effects.
	appendX := func(data []byte) []byte {
		return append(data, 'X')
	}
	appendY := func(data []byte) []byte {
		return append(data, 'Y')
	}

	tests := []struct {
		name  string
		fns   []testutils.NormalizeFunc
		input string
		want  string
	}{
		{
			name:  "zero-functions-unchanged",
			fns:   nil,
			input: "hello",
			want:  "hello",
		},
		{
			name:  "single-function",
			input: "hello",
			fns:   []testutils.NormalizeFunc{appendX},
			want:  "helloX",
		},
		{
			name:  "two-functions-in-order",
			input: "hello",
			fns:   []testutils.NormalizeFunc{appendX, appendY},
			want:  "helloXY",
		},
		{
			name:  "reverse-order-differs",
			input: "hello",
			fns:   []testutils.NormalizeFunc{appendY, appendX},
			want:  "helloYX",
		},
		{
			name:  "empty-input",
			input: "",
			fns:   []testutils.NormalizeFunc{appendX, appendY},
			want:  "XY",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			composed := testutils.ComposeNormalizers(tc.fns...)
			got := composed([]byte(tc.input))
			if !bytes.Equal(got, []byte(tc.want)) {
				t.Fatalf("ComposeNormalizers(...)(%q)\ngot:  %q\nwant: %q", tc.input, got, tc.want)
			}
		})
	}
}
