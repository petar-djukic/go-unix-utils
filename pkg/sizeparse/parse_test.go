// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package sizeparse

import (
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// R1.4: bare numeric, no suffix
		{"bare zero", "0", 0, false},
		{"bare number", "42", 42, false},
		// R1.2: binary (IEC) suffixes
		{"K suffix", "100K", 102400, false},
		{"KiB suffix", "100KiB", 102400, false},
		{"M suffix", "1M", 1048576, false},
		{"MiB suffix", "1MiB", 1048576, false},
		{"G suffix", "2G", 2147483648, false},
		{"GiB suffix", "2GiB", 2147483648, false},
		{"T suffix", "1T", 1099511627776, false},
		{"TiB suffix", "1TiB", 1099511627776, false},
		{"P suffix", "1P", 1125899906842624, false},
		{"PiB suffix", "1PiB", 1125899906842624, false},
		{"E suffix", "1E", 1152921504606846976, false},
		{"EiB suffix", "1EiB", 1152921504606846976, false},
		{"b suffix", "8b", 4096, false},
		// R1.2: lowercase single-letter binary suffixes
		{"lowercase k", "100k", 102400, false},
		{"lowercase m", "1m", 1048576, false},
		{"lowercase g", "2g", 2147483648, false},
		{"lowercase t", "1t", 1099511627776, false},
		{"lowercase p", "1p", 1125899906842624, false},
		{"lowercase e", "1e", 1152921504606846976, false},
		// R1.3: decimal (SI) suffixes
		{"KB suffix", "5KB", 5000, false},
		{"MB suffix", "5MB", 5000000, false},
		{"GB suffix", "1GB", 1000000000, false},
		{"TB suffix", "1TB", 1000000000000, false},
		{"PB suffix", "1PB", 1000000000000000, false},
		{"EB suffix", "1EB", 1000000000000000000, false},
		// R1.2/R1.3: Z and Y overflow int64 for n > 0
		{"Z overflow", "1Z", 0, true},
		{"ZiB overflow", "1ZiB", 0, true},
		{"ZB overflow", "1ZB", 0, true},
		{"Y overflow", "1Y", 0, true},
		{"YiB overflow", "1YiB", 0, true},
		{"YB overflow", "1YB", 0, true},
		// Z/Y with 0 are valid
		{"0Z", "0Z", 0, false},
		{"0Y", "0Y", 0, false},
		{"0ZB", "0ZB", 0, false},
		{"0YB", "0YB", 0, false},
		// R3.2: unknown suffix
		{"unknown suffix", "10X", 0, true},
		{"unknown suffix word", "10foo", 0, true},
		// R3.1: empty and invalid
		{"empty string", "", 0, true},
		{"non-numeric", "invalid", 0, true},
		{"only suffix", "K", 0, true},
		// R3.1: overflow
		{"overflow E", "9999999999999999999E", 0, true},
		// Parse does not allow signs by default (AllowSign=false)
		{"plus rejected", "+100K", 0, true},
		{"minus rejected", "-50M", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) = %d, want error", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("Parse(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestParseWithOptionsAllowSign(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		opts    ParseOptions
		want    int64
		wantErr bool
	}{
		// R1.4 + R2.1: positive sign
		{"plus K", "+100K", ParseOptions{AllowSign: true}, 102400, false},
		// R1.4 + R2.1: negative sign
		{"minus M", "-50M", ParseOptions{AllowSign: true}, -52428800, false},
		// AC3: explicit acceptance criterion
		{"AC3 plus 50M", "+50M", ParseOptions{AllowSign: true}, 52428800, false},
		// no sign with AllowSign=true is fine
		{"no sign allowed", "100K", ParseOptions{AllowSign: true}, 102400, false},
		// sign not allowed errors
		{"sign not allowed", "+100K", ParseOptions{AllowSign: false}, 0, true},
		// bare "+" or "-" alone
		{"plus only", "+", ParseOptions{AllowSign: true}, 0, true},
		{"minus only", "-", ParseOptions{AllowSign: true}, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseWithOptions(tc.input, tc.opts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseWithOptions(%q) = %d, want error",
						tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseWithOptions(%q) unexpected error: %v",
					tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseWithOptions(%q) = %d, want %d",
					tc.input, got, tc.want)
			}
		})
	}
}

func TestParseWithOptionsDefaultUnit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		opts  ParseOptions
		want  int64
	}{
		// R2.2: DefaultUnit=1024, bare "5" → 5120
		{"default unit 1024", "5", ParseOptions{DefaultUnit: 1024}, 5120},
		// DefaultUnit=512 (block size)
		{"default unit 512", "10", ParseOptions{DefaultUnit: 512}, 5120},
		// DefaultUnit ignored when suffix present
		{"suffix overrides default", "5K",
			ParseOptions{DefaultUnit: 512}, 5120},
		// DefaultUnit=0 falls back to 1
		{"default unit zero", "42", ParseOptions{DefaultUnit: 0}, 42},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseWithOptions(tc.input, tc.opts)
			if err != nil {
				t.Fatalf("ParseWithOptions(%q) unexpected error: %v",
					tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseWithOptions(%q) = %d, want %d",
					tc.input, got, tc.want)
			}
		})
	}
}
