// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Tests for HumanSize (prd003-format R3.1, R3.2, R3.3, R3.4).
package format

import (
	"testing"
)

func TestHumanSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		opts  HumanSizeOpts
		want  string
	}{
		// R3.4: zero bytes returns "0" regardless of mode.
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

		// Below first unit threshold: plain integer without suffix.
		{
			name:  "binary below 1024",
			bytes: 512,
			opts:  HumanSizeOpts{Binary: true},
			want:  "512",
		},
		{
			name:  "SI below 1000",
			bytes: 999,
			opts:  HumanSizeOpts{Binary: false},
			want:  "999",
		},
		{
			name:  "binary one byte",
			bytes: 1,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1",
		},

		// R3.1, R3.2: binary mode (1024-based) with K/M/G/T/P/E suffixes.
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
			name:  "binary 10K no decimal",
			bytes: 10240,
			opts:  HumanSizeOpts{Binary: true},
			want:  "10K",
		},
		{
			name:  "binary exactly 1M",
			bytes: 1048576,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0M",
		},
		{
			name:  "binary exactly 1G",
			bytes: 1073741824,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0G",
		},
		{
			name:  "binary large value T",
			bytes: 1099511627776,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0T",
		},
		{
			name:  "binary large value P",
			bytes: 1125899906842624,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0P",
		},
		{
			name:  "binary large value E",
			bytes: 1152921504606846976,
			opts:  HumanSizeOpts{Binary: true},
			want:  "1.0E",
		},

		// R3.1, R3.2: SI mode (1000-based) with kB/MB/GB/TB suffixes.
		{
			name:  "SI exactly 1kB",
			bytes: 1000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "1.0kB",
		},
		{
			name:  "SI 1.0MB",
			bytes: 1000000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "1.0MB",
		},
		{
			name:  "SI 10MB no decimal",
			bytes: 10000000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "10MB",
		},
		{
			name:  "SI 1.0GB",
			bytes: 1000000000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "1.0GB",
		},
		{
			name:  "SI 1.0TB",
			bytes: 1000000000000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "1.0TB",
		},

		// R3.3: one decimal place for values below 10, no decimal for >= 10.
		{
			name:  "binary 9.9K has decimal",
			bytes: 10137, // 10137/1024 ≈ 9.9
			opts:  HumanSizeOpts{Binary: true},
			want:  "9.9K",
		},
		{
			name:  "SI 5.0kB has decimal",
			bytes: 5000,
			opts:  HumanSizeOpts{Binary: false},
			want:  "5.0kB",
		},
		{
			name:  "binary 100M no decimal",
			bytes: 104857600, // 100 * 1024^2
			opts:  HumanSizeOpts{Binary: true},
			want:  "100M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HumanSize(tt.bytes, tt.opts)
			if got != tt.want {
				t.Errorf("HumanSize(%d, {Binary: %v}) = %q; want %q",
					tt.bytes, tt.opts.Binary, got, tt.want)
			}
		})
	}
}
