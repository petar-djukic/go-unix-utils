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
		// R3.4: zero is always "0" regardless of mode.
		{"si zero", 0, HumanSizeOpts{Binary: false}, "0"},
		{"binary zero", 0, HumanSizeOpts{Binary: true}, "0"},

		// SI mode (base 1000): boundary values.
		{"si 1 byte", 1, HumanSizeOpts{Binary: false}, "1"},
		{"si 500 bytes", 500, HumanSizeOpts{Binary: false}, "500"},
		{"si 999 bytes", 999, HumanSizeOpts{Binary: false}, "999"},
		{"si 1000 boundary", 1000, HumanSizeOpts{Binary: false}, "1.0K"},
		{"si 1001", 1001, HumanSizeOpts{Binary: false}, "1.0K"},
		{"si 1500 (1.5K)", 1500, HumanSizeOpts{Binary: false}, "1.5K"},
		{"si 1M", 1000000, HumanSizeOpts{Binary: false}, "1.0M"},
		{"si 1.5M", 1500000, HumanSizeOpts{Binary: false}, "1.5M"},
		{"si 1G", 1000000000, HumanSizeOpts{Binary: false}, "1.0G"},
		{"si 1T", 1000000000000, HumanSizeOpts{Binary: false}, "1.0T"},
		{"si 2.5T", 2500000000000, HumanSizeOpts{Binary: false}, "2.5T"},

		// Binary mode (base 1024): boundary values.
		{"binary 1 byte", 1, HumanSizeOpts{Binary: true}, "1"},
		{"binary 512 bytes", 512, HumanSizeOpts{Binary: true}, "512"},
		{"binary 1023 boundary", 1023, HumanSizeOpts{Binary: true}, "1023"},
		{"binary 1024 boundary", 1024, HumanSizeOpts{Binary: true}, "1.0Ki"},
		{"binary 1025", 1025, HumanSizeOpts{Binary: true}, "1.0Ki"},
		{"binary 1536 (1.5Ki)", 1536, HumanSizeOpts{Binary: true}, "1.5Ki"},
		{"binary 1Mi", 1048576, HumanSizeOpts{Binary: true}, "1.0Mi"},
		{"binary 1.5Mi", 1572864, HumanSizeOpts{Binary: true}, "1.5Mi"},
		{"binary 1Gi", 1073741824, HumanSizeOpts{Binary: true}, "1.0Gi"},
		{"binary 1Ti", 1099511627776, HumanSizeOpts{Binary: true}, "1.0Ti"},
		{"binary 2.5Gi", 2684354560, HumanSizeOpts{Binary: true}, "2.5Gi"},
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
