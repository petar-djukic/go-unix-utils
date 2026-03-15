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
		// AC2: zero returns "0"
		{"zero SI", 0, HumanSizeOpts{Binary: false}, "0"},
		{"zero binary", 0, HumanSizeOpts{Binary: true}, "0"},

		// R1.4: below threshold returns exact byte count
		{"below SI threshold", 999, HumanSizeOpts{Binary: false}, "999"},
		{"below binary threshold", 1023, HumanSizeOpts{Binary: true}, "1023"},

		// AC3: fractional values
		{"1500 SI", 1500, HumanSizeOpts{Binary: false}, "1.5K"},
		{"1536 binary", 1536, HumanSizeOpts{Binary: true}, "1.5Ki"},

		// AC4: exact unit boundary drops .0
		{"1000 SI", 1000, HumanSizeOpts{Binary: false}, "1K"},
		{"1024 binary", 1024, HumanSizeOpts{Binary: true}, "1Ki"},
		{"1048576 binary", 1048576, HumanSizeOpts{Binary: true}, "1Mi"},
		{"1000000 SI", 1000000, HumanSizeOpts{Binary: false}, "1M"},

		// larger units
		{"1073741824 binary", 1073741824, HumanSizeOpts{Binary: true}, "1Gi"},
		{"1000000000 SI", 1000000000, HumanSizeOpts{Binary: false}, "1G"},

		// tera
		{"1099511627776 binary", 1099511627776, HumanSizeOpts{Binary: true}, "1Ti"},
		{"1000000000000 SI", 1000000000000, HumanSizeOpts{Binary: false}, "1T"},

		// fractional larger units
		{"1500000 SI", 1500000, HumanSizeOpts{Binary: false}, "1.5M"},
		{"1572864 binary", 1572864, HumanSizeOpts{Binary: true}, "1.5Mi"},

		// single byte
		{"1 byte SI", 1, HumanSizeOpts{Binary: false}, "1"},
		{"1 byte binary", 1, HumanSizeOpts{Binary: true}, "1"},

		// boundary: exactly at threshold
		{"exactly 1000 SI", 1000, HumanSizeOpts{Binary: false}, "1K"},
		{"exactly 1024 binary", 1024, HumanSizeOpts{Binary: true}, "1Ki"},

		// large value that exercises highest suffix
		{"1 exbi binary", 1152921504606846976, HumanSizeOpts{Binary: true}, "1Ei"},
		{"1 exa SI", 1000000000000000000, HumanSizeOpts{Binary: false}, "1E"},

		// rounding at decimal
		{"1126 binary rounds to 1.1Ki", 1126, HumanSizeOpts{Binary: true}, "1.1Ki"},
		{"1100 SI rounds to 1.1K", 1100, HumanSizeOpts{Binary: false}, "1.1K"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, tc.opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, {Binary: %v}) = %q, want %q", tc.bytes, tc.opts.Binary, got, tc.want)
			}
		})
	}
}
