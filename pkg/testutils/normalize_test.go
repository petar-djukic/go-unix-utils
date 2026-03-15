// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package testutils

import (
	"bytes"
	"testing"
)

func TestComposeNormalizers_ZeroFunctions(t *testing.T) {
	t.Parallel()

	// AC1: zero functions returns input bytes unchanged.
	fn := ComposeNormalizers()
	input := []byte("hello world")
	got := fn(input)
	if !bytes.Equal(got, input) {
		t.Errorf("expected %q, got %q", input, got)
	}
}

func TestComposeNormalizers_SingleFunction(t *testing.T) {
	t.Parallel()

	upper := func(b []byte) []byte { return bytes.ToUpper(b) }
	fn := ComposeNormalizers(upper)
	input := []byte("hello")
	got := fn(input)
	want := []byte("HELLO")
	if !bytes.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestComposeNormalizers_MultipleFunctions(t *testing.T) {
	t.Parallel()

	// AC2: multiple functions applied left-to-right.
	addPrefix := func(b []byte) []byte { return append([]byte("prefix:"), b...) }
	upper := func(b []byte) []byte { return bytes.ToUpper(b) }

	fn := ComposeNormalizers(addPrefix, upper)
	input := []byte("hello")
	got := fn(input)
	want := []byte("PREFIX:HELLO")
	if !bytes.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestComposeNormalizers_NilFunctionsSkipped(t *testing.T) {
	t.Parallel()

	// AC3: nil functions in the variadic list are skipped without panicking.
	upper := func(b []byte) []byte { return bytes.ToUpper(b) }
	fn := ComposeNormalizers(nil, upper, nil)
	input := []byte("hello")
	got := fn(input)
	want := []byte("HELLO")
	if !bytes.Equal(got, want) {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestComposeNormalizers_AllNil(t *testing.T) {
	t.Parallel()

	// All nil functions should still return input unchanged.
	fn := ComposeNormalizers(nil, nil, nil)
	input := []byte("unchanged")
	got := fn(input)
	if !bytes.Equal(got, input) {
		t.Errorf("expected %q, got %q", input, got)
	}
}

func TestTimestampNormalizer_ISO8601(t *testing.T) {
	t.Parallel()

	// AC4: replaces ISO 8601 timestamps.
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"date_space_time", "log 2026-03-14 12:34:56 event", "log <TIMESTAMP> event"},
		{"date_T_time", "log 2026-03-14T12:34:56 event", "log <TIMESTAMP> event"},
		{"with_fractional", "log 2026-03-14T12:34:56.789 event", "log <TIMESTAMP> event"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := TimestampNormalizer([]byte(tc.input))
			if string(got) != tc.want {
				t.Errorf("expected %q, got %q", tc.want, string(got))
			}
		})
	}
}

func TestTimestampNormalizer_EpochSeconds(t *testing.T) {
	t.Parallel()

	// AC5: replaces floating-point epoch seconds.
	input := []byte("timestamp: 1710412496.123456 end")
	got := TimestampNormalizer(input)
	want := "timestamp: <TIMESTAMP> end"
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}
}

func TestTimestampNormalizer_CtimeFormat(t *testing.T) {
	t.Parallel()

	input := []byte("Mar 14 12:34:56 syslog")
	got := TimestampNormalizer(input)
	want := "<TIMESTAMP> syslog"
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}
}

func TestTimestampNormalizer_LsYearFormat(t *testing.T) {
	t.Parallel()

	input := []byte("Mar 14  2026")
	got := TimestampNormalizer(input)
	want := "<TIMESTAMP>"
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}
}

func TestTimestampNormalizer_HHMMSSFormat(t *testing.T) {
	t.Parallel()

	input := []byte("time: 12:34:56 end")
	got := TimestampNormalizer(input)
	want := "time: <TIMESTAMP> end"
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}
}

func TestTimestampNormalizer_NoMatch(t *testing.T) {
	t.Parallel()

	input := []byte("no timestamps here")
	got := TimestampNormalizer(input)
	if !bytes.Equal(got, input) {
		t.Errorf("expected %q, got %q", input, got)
	}
}
