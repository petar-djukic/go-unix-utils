// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "testing"

func TestHumanSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		opts  HumanSizeOpts
		want  string
	}{
		// AC1: zero returns "0"
		{"zero_si", 0, HumanSizeOpts{}, "0"},
		{"zero_binary", 0, HumanSizeOpts{Binary: true}, "0"},

		// AC4: below threshold — plain integer, no suffix
		{"small_si", 999, HumanSizeOpts{}, "999"},
		{"small_binary", 1023, HumanSizeOpts{Binary: true}, "1023"},
		{"one_byte", 1, HumanSizeOpts{}, "1"},

		// AC2: SI units (1000-based)
		{"1500_si", 1500, HumanSizeOpts{}, "1.5K"},
		{"1000_si", 1000, HumanSizeOpts{}, "1.0K"},
		{"1000000_si", 1000000, HumanSizeOpts{}, "1.0M"},
		{"1500000_si", 1500000, HumanSizeOpts{}, "1.5M"},
		{"1000000000_si", 1000000000, HumanSizeOpts{}, "1.0G"},
		{"1000000000000_si", 1000000000000, HumanSizeOpts{}, "1.0T"},

		// AC3: IEC units (1024-based)
		{"1536_binary", 1536, HumanSizeOpts{Binary: true}, "1.5Ki"},
		{"1024_binary", 1024, HumanSizeOpts{Binary: true}, "1.0Ki"},
		{"1048576_binary", 1048576, HumanSizeOpts{Binary: true}, "1.0Mi"},
		{"1073741824_binary", 1073741824, HumanSizeOpts{Binary: true}, "1.0Gi"},

		// R3.3: precision — values >= 100 use integer display
		{"150000_si", 150000, HumanSizeOpts{}, "150K"},
		{"999999_si", 999999, HumanSizeOpts{}, "1000K"},

		// R1.4: negative values
		{"negative_si", -1500, HumanSizeOpts{}, "-1.5K"},
		{"negative_binary", -1536, HumanSizeOpts{Binary: true}, "-1.5Ki"},

		// R1.4: exact unit boundaries
		{"exact_1M_si", 1000000, HumanSizeOpts{}, "1.0M"},
		{"exact_1G_binary", 1073741824, HumanSizeOpts{Binary: true}, "1.0Gi"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, tc.opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, %+v) = %q, want %q", tc.bytes, tc.opts, got, tc.want)
			}
		})
	}
}
