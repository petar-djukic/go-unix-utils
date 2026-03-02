// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for size.go: HumanSize in binary and SI modes.
// Implements: prd003-format (R3)
package format

import "testing"

func TestHumanSize(t *testing.T) {
	tests := []struct {
		name   string
		bytes  int64
		opts   HumanSizeOpts
		want   string
	}{
		// Zero bytes (prd003-format R3.4)
		{
			name:  "zero bytes binary",
			bytes: 0,
			opts:  HumanSizeOpts{Binary: true},
			want:  "0",
		},
		{
			name:  "zero bytes SI",
			bytes: 0,
			opts:  HumanSizeOpts{Binary: false},
			want:  "0",
		},

		// Binary mode (base 1024) (prd003-format R3.2)
		{
			name:  "binary sub-K",
			bytes: 512,
			opts:  HumanSizeOpts{Binary: true},
			want:  "512",
		},
		{
			name:  "binary exactly 1K",
			bytes: 1024,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0K",
		},
		{
			name:  "binary 1.5K",
			bytes: 1536,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.5K",
		},
		{
			name:  "binary exactly 1M",
			bytes: 1024 * 1024,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0M",
		},
		{
			name:  "binary exactly 1G",
			bytes: 1024 * 1024 * 1024,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0G",
		},
		{
			name:  "binary exactly 1T",
			bytes: 1024 * 1024 * 1024 * 1024,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0T",
		},
		{
			name:  "binary exactly 1P",
			bytes: 1024 * 1024 * 1024 * 1024 * 1024,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0P",
		},
		{
			name:  "binary exactly 1E",
			bytes: 1024 * 1024 * 1024 * 1024 * 1024 * 1024,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0E",
		},
		{
			name:  "binary 4K",
			bytes: 4096,
			opts:  HumanSizeOpts{Binary: true},
			want:  "4.0K",
		},

		// SI mode (base 1000) (prd003-format R3.2)
		{
			name:  "SI sub-kB",
			bytes: 500,
			opts:  HumanSizeOpts{Binary: false},
			want:  "500",
		},
		{
			name:  "SI exactly 1kB",
			bytes: 1000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "1.0kB",
		},
		{
			name:  "SI 1MB",
			bytes: 1000000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "1.0MB",
		},
		{
			name:  "SI 1GB",
			bytes: 1000000000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "1.0GB",
		},
		{
			name:  "SI 1TB",
			bytes: 1000000000000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "1.0TB",
		},
		{
			name:  "SI 1.5kB",
			bytes: 1500,
			opts:  HumanSizeOpts{Binary: false},
			want:  "1.5kB",
		},

		// Decimal formatting (prd003-format R3.3)
		{
			name:  "binary fractional value",
			bytes: 1536, // 1.5K
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.5K",
		},
		{
			name:  "SI fractional value",
			bytes: 2500,
			opts:  HumanSizeOpts{Binary: false},
			want:  "2.5kB",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HumanSize(tc.bytes, tc.opts)
			if got != tc.want {
				t.Fatalf("HumanSize(%d, {Binary: %v}) = %q, want %q", tc.bytes, tc.opts.Binary, got, tc.want)
			}
		})
	}
}
