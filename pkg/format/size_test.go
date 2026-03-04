// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for prd003-format R3.1-R3.4: HumanSize, HumanSizeOpts.

package format_test

import (
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
)

// TestHumanSize_Zero verifies that zero bytes returns "0" for both Binary
// and SI modes.
// R3.4: zero returns "0" regardless of unit mode.
func TestHumanSize_Zero(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts format.HumanSizeOpts
	}{
		{"binary", format.HumanSizeOpts{Binary: true}},
		{"si", format.HumanSizeOpts{Binary: false}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := format.HumanSize(0, tc.opts)
			if got != "0" {
				t.Errorf("HumanSize(0, %+v) = %q, want %q", tc.opts, got, "0")
			}
		})
	}
}

// TestHumanSize_Binary verifies 1024-based conversion across all unit
// boundaries from bytes through E suffix.
// R3.1, R3.2: base-1024 with suffixes K/M/G/T/P/E.
func TestHumanSize_Binary(t *testing.T) {
	t.Parallel()

	opts := format.HumanSizeOpts{Binary: true}
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		// Bytes (no suffix) — R3.2: values below base return integer with no suffix.
		{"1_byte", 1, "1"},
		{"512_bytes", 512, "512"},
		{"1023_bytes", 1023, "1023"},

		// K suffix — exact boundary and above.
		{"1K_exact", 1024, "1.0K"},
		{"1.5K", 1536, "1.5K"},
		{"10K", 10240, "10K"},
		{"999K", 1022976, "999K"},

		// M suffix.
		{"1M_exact", 1048576, "1.0M"},
		{"1.5M", 1572864, "1.5M"},
		{"10M", 10485760, "10M"},

		// G suffix.
		{"1G_exact", 1073741824, "1.0G"},
		{"2.5G", 2684354560, "2.5G"},
		{"10G", 10737418240, "10G"},

		// T suffix.
		{"1T_exact", 1099511627776, "1.0T"},
		{"15T", 16492674416640, "15T"},

		// P suffix.
		{"1P_exact", 1125899906842624, "1.0P"},
		{"12P", 13510798882111488, "12P"},

		// E suffix.
		{"1E_exact", 1152921504606846976, "1.0E"},
		{"4E", 4611686018427387904, "4.0E"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := format.HumanSize(tc.bytes, opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, binary) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

// TestHumanSize_SI verifies 1000-based conversion across all unit boundaries
// from bytes through TB suffix.
// R3.1, R3.2: base-1000 with suffixes kB/MB/GB/TB.
func TestHumanSize_SI(t *testing.T) {
	t.Parallel()

	opts := format.HumanSizeOpts{Binary: false}
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		// Bytes (no suffix) — R3.2: values below base return integer with no suffix.
		{"1_byte", 1, "1"},
		{"500_bytes", 500, "500"},
		{"999_bytes", 999, "999"},

		// kB suffix.
		{"1kB_exact", 1000, "1.0kB"},
		{"1.5kB", 1500, "1.5kB"},
		{"10kB", 10000, "10kB"},
		{"999kB", 999000, "999kB"},

		// MB suffix.
		{"1MB_exact", 1000000, "1.0MB"},
		{"5.5MB", 5500000, "5.5MB"},
		{"10MB", 10000000, "10MB"},

		// GB suffix.
		{"1GB_exact", 1000000000, "1.0GB"},
		{"2.3GB", 2300000000, "2.3GB"},
		{"10GB", 10000000000, "10GB"},

		// TB suffix.
		{"1TB_exact", 1000000000000, "1.0TB"},
		{"15TB", 15000000000000, "15TB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := format.HumanSize(tc.bytes, opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, SI) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

// TestHumanSize_DecimalFormatting verifies that values below 10 at the chosen
// unit produce one decimal place and values 10 or above produce no decimal.
// R3.3: at most one decimal place when value < 10.
func TestHumanSize_DecimalFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		opts  format.HumanSizeOpts
		want  string
	}{
		// Binary: below 10 at K → one decimal place.
		{"binary_1.5K", 1536, format.HumanSizeOpts{Binary: true}, "1.5K"},
		{"binary_9.5K", 9728, format.HumanSizeOpts{Binary: true}, "9.5K"},
		// Binary: at or above 10 at K → no decimal place.
		{"binary_10K", 10240, format.HumanSizeOpts{Binary: true}, "10K"},
		{"binary_50K", 51200, format.HumanSizeOpts{Binary: true}, "50K"},

		// SI: below 10 at kB → one decimal place.
		{"si_1.5kB", 1500, format.HumanSizeOpts{Binary: false}, "1.5kB"},
		{"si_9.9kB", 9900, format.HumanSizeOpts{Binary: false}, "9.9kB"},
		// SI: at or above 10 at kB → no decimal place.
		{"si_10kB", 10000, format.HumanSizeOpts{Binary: false}, "10kB"},
		{"si_50kB", 50000, format.HumanSizeOpts{Binary: false}, "50kB"},

		// Binary: below 10 at M → one decimal place.
		{"binary_1.0M", 1048576, format.HumanSizeOpts{Binary: true}, "1.0M"},
		// Binary: above 10 at M → no decimal place.
		{"binary_10M", 10485760, format.HumanSizeOpts{Binary: true}, "10M"},

		// SI: below 10 at MB → one decimal place.
		{"si_1.0MB", 1000000, format.HumanSizeOpts{Binary: false}, "1.0MB"},
		// SI: above 10 at MB → no decimal place.
		{"si_10MB", 10000000, format.HumanSizeOpts{Binary: false}, "10MB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := format.HumanSize(tc.bytes, tc.opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, %+v) = %q, want %q", tc.bytes, tc.opts, got, tc.want)
			}
		})
	}
}

// TestHumanSize_BelowBase verifies that values below the base (< 1024 for
// binary, < 1000 for SI) return integer format with no suffix.
// R3.2: no suffix for sub-base values.
func TestHumanSize_BelowBase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		opts  format.HumanSizeOpts
		want  string
	}{
		{"binary_1", 1, format.HumanSizeOpts{Binary: true}, "1"},
		{"binary_100", 100, format.HumanSizeOpts{Binary: true}, "100"},
		{"binary_1023", 1023, format.HumanSizeOpts{Binary: true}, "1023"},
		{"si_1", 1, format.HumanSizeOpts{Binary: false}, "1"},
		{"si_100", 100, format.HumanSizeOpts{Binary: false}, "100"},
		{"si_999", 999, format.HumanSizeOpts{Binary: false}, "999"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := format.HumanSize(tc.bytes, tc.opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, %+v) = %q, want %q", tc.bytes, tc.opts, got, tc.want)
			}
		})
	}
}

// TestHumanSize_ExactBoundaries verifies correct suffix selection at exact
// unit thresholds (1024^n for binary, 1000^n for SI).
// R3.1: correct unit boundary transitions.
func TestHumanSize_ExactBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		opts  format.HumanSizeOpts
		want  string
	}{
		// Binary boundaries.
		{"binary_1024", 1024, format.HumanSizeOpts{Binary: true}, "1.0K"},
		{"binary_1048576", 1048576, format.HumanSizeOpts{Binary: true}, "1.0M"},
		{"binary_1073741824", 1073741824, format.HumanSizeOpts{Binary: true}, "1.0G"},
		{"binary_1099511627776", 1099511627776, format.HumanSizeOpts{Binary: true}, "1.0T"},
		{"binary_1125899906842624", 1125899906842624, format.HumanSizeOpts{Binary: true}, "1.0P"},
		{"binary_1152921504606846976", 1152921504606846976, format.HumanSizeOpts{Binary: true}, "1.0E"},

		// SI boundaries.
		{"si_1000", 1000, format.HumanSizeOpts{Binary: false}, "1.0kB"},
		{"si_1000000", 1000000, format.HumanSizeOpts{Binary: false}, "1.0MB"},
		{"si_1000000000", 1000000000, format.HumanSizeOpts{Binary: false}, "1.0GB"},
		{"si_1000000000000", 1000000000000, format.HumanSizeOpts{Binary: false}, "1.0TB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := format.HumanSize(tc.bytes, tc.opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, %+v) = %q, want %q", tc.bytes, tc.opts, got, tc.want)
			}
		})
	}
}
