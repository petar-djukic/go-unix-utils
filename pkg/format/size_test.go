// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for human-readable size formatting.
//
// Implements: prd003-format R3.1, R3.2, R3.3, R3.4
package format

import (
	"testing"
)

// --- HumanSize binary mode tests (prd003-format R3.1, R3.2, R3.3, R3.4) ---

func TestHumanSize_Binary(t *testing.T) {
	tests := []struct {
		name   string
		bytes  int64
		expect string
	}{
		{
			name:   "zero",
			bytes:  0,
			expect: "0",
		},
		{
			name:   "bytes-below-1024",
			bytes:  512,
			expect: "512",
		},
		{
			name:   "exactly-1024-is-1K",
			bytes:  1024,
			expect: "1.0K",
		},
		{
			name:   "1536-is-1.5K",
			bytes:  1536,
			expect: "1.5K",
		},
		{
			name:   "10240-is-10K",
			bytes:  10240,
			expect: "10K",
		},
		{
			name:   "one-megabyte",
			bytes:  1048576,
			expect: "1.0M",
		},
		{
			name:   "1.5-megabytes",
			bytes:  1572864,
			expect: "1.5M",
		},
		{
			name:   "10-megabytes",
			bytes:  10485760,
			expect: "10M",
		},
		{
			name:   "one-gigabyte",
			bytes:  1073741824,
			expect: "1.0G",
		},
		{
			name:   "one-terabyte",
			bytes:  1099511627776,
			expect: "1.0T",
		},
		{
			name:   "one-petabyte",
			bytes:  1125899906842624,
			expect: "1.0P",
		},
		{
			name:   "one-exabyte",
			bytes:  1152921504606846976,
			expect: "1.0E",
		},
		{
			name:   "value-under-10-shows-decimal",
			bytes:  5120,
			expect: "5.0K",
		},
		{
			name:   "value-at-10-shows-integer",
			bytes:  10240,
			expect: "10K",
		},
		{
			name:   "value-over-10-shows-integer",
			bytes:  102400,
			expect: "100K",
		},
		{
			name:   "single-byte",
			bytes:  1,
			expect: "1",
		},
	}

	opts := HumanSizeOpts{Binary: true}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HumanSize(tc.bytes, opts)
			if got != tc.expect {
				t.Errorf("HumanSize(%d, Binary) = %q, want %q", tc.bytes, got, tc.expect)
			}
		})
	}
}

// --- HumanSize SI mode tests (prd003-format R3.1, R3.3, R3.4) ---

func TestHumanSize_SI(t *testing.T) {
	tests := []struct {
		name   string
		bytes  int64
		expect string
	}{
		{
			name:   "zero",
			bytes:  0,
			expect: "0",
		},
		{
			name:   "bytes-below-1000",
			bytes:  500,
			expect: "500",
		},
		{
			name:   "exactly-1000-is-1.0kB",
			bytes:  1000,
			expect: "1.0kB",
		},
		{
			name:   "1000000-is-1.0MB",
			bytes:  1000000,
			expect: "1.0MB",
		},
		{
			name:   "1500-is-1.5kB",
			bytes:  1500,
			expect: "1.5kB",
		},
		{
			name:   "10000-is-10kB",
			bytes:  10000,
			expect: "10kB",
		},
		{
			name:   "one-gigabyte-SI",
			bytes:  1000000000,
			expect: "1.0GB",
		},
		{
			name:   "one-terabyte-SI",
			bytes:  1000000000000,
			expect: "1.0TB",
		},
		{
			name:   "value-under-10-shows-decimal",
			bytes:  5000,
			expect: "5.0kB",
		},
		{
			name:   "value-over-10-shows-integer",
			bytes:  50000,
			expect: "50kB",
		},
		{
			name:   "single-byte",
			bytes:  1,
			expect: "1",
		},
	}

	opts := HumanSizeOpts{Binary: false}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HumanSize(tc.bytes, opts)
			if got != tc.expect {
				t.Errorf("HumanSize(%d, SI) = %q, want %q", tc.bytes, got, tc.expect)
			}
		})
	}
}
