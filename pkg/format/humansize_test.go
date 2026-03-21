// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "testing"

func TestHumanSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		bytes  int64
		opts   HumanSizeOpts
		want   string
	}{
		// R3.4: zero returns "0" regardless of mode.
		{"zero_binary", 0, HumanSizeOpts{Binary: true}, "0"},
		{"zero_si", 0, HumanSizeOpts{Binary: false}, "0"},

		// R3.2: Binary mode (1024-based, suffixes K/M/G/T/P/E).
		{"binary_sub_unit", 512, HumanSizeOpts{Binary: true}, "512"},
		{"binary_1K", 1024, HumanSizeOpts{Binary: true}, "1K"},
		{"binary_1.5K", 1536, HumanSizeOpts{Binary: true}, "1.5K"},
		{"binary_1M", 1048576, HumanSizeOpts{Binary: true}, "1M"},
		{"binary_1.5M", 1572864, HumanSizeOpts{Binary: true}, "1.5M"},
		{"binary_1G", 1073741824, HumanSizeOpts{Binary: true}, "1G"},
		{"binary_2.5G", 2684354560, HumanSizeOpts{Binary: true}, "2.5G"},
		{"binary_1T", 1099511627776, HumanSizeOpts{Binary: true}, "1T"},

		// R3.2: SI mode (1000-based, suffixes kB/MB/GB/TB).
		{"si_sub_unit", 999, HumanSizeOpts{Binary: false}, "999"},
		{"si_1kB", 1000, HumanSizeOpts{Binary: false}, "1kB"},
		{"si_1.5kB", 1500, HumanSizeOpts{Binary: false}, "1.5kB"},
		{"si_1MB", 1000000, HumanSizeOpts{Binary: false}, "1MB"},
		{"si_1.5MB", 1500000, HumanSizeOpts{Binary: false}, "1.5MB"},
		{"si_1GB", 1000000000, HumanSizeOpts{Binary: false}, "1GB"},
		{"si_1TB", 1000000000000, HumanSizeOpts{Binary: false}, "1TB"},

		// Negative values preserve sign.
		{"negative_binary", -1536, HumanSizeOpts{Binary: true}, "-1.5K"},
		{"negative_si", -1500, HumanSizeOpts{Binary: false}, "-1.5kB"},
		{"negative_sub_unit", -500, HumanSizeOpts{Binary: true}, "-500"},

		// Single byte.
		{"one_byte", 1, HumanSizeOpts{Binary: true}, "1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, tc.opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, {Binary: %v}) = %q, want %q",
					tc.bytes, tc.opts.Binary, got, tc.want)
			}
		})
	}
}
