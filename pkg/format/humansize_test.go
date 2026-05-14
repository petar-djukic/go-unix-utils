// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "testing"

func TestHumanSizeBinary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0"},
		{"small_no_suffix", 512, "512"},
		{"exact_K", 1024, "1.0K"},
		{"fractional_K", 1536, "1.5K"},
		{"ten_K", 10240, "10K"},
		{"exact_M", 1048576, "1.0M"},
		{"exact_G", 1073741824, "1.0G"},
		{"exact_T", 1099511627776, "1.0T"},
	}
	opts := HumanSizeOpts{Binary: true}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HumanSize(tt.bytes, opts); got != tt.want {
				t.Errorf("HumanSize(%d, binary) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestHumanSizeSI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0"},
		{"small_no_suffix", 500, "500"},
		{"exact_kB", 1000, "1.0kB"},
		{"fractional_kB", 1500, "1.5kB"},
		{"exact_MB", 1000000, "1.0MB"},
		{"exact_GB", 1000000000, "1.0GB"},
	}
	opts := HumanSizeOpts{Binary: false}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := HumanSize(tt.bytes, opts); got != tt.want {
				t.Errorf("HumanSize(%d, SI) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}
