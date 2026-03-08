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
		// AC1: boundary values in binary mode
		{"zero binary", 0, HumanSizeOpts{Binary: true}, "0"},
		{"999 binary", 999, HumanSizeOpts{Binary: true}, "999"},
		{"1000 binary", 1000, HumanSizeOpts{Binary: true}, "1000"},
		{"1023 binary", 1023, HumanSizeOpts{Binary: true}, "1023"},
		{"1024 binary", 1024, HumanSizeOpts{Binary: true}, "1K"},
		{"1536 binary", 1536, HumanSizeOpts{Binary: true}, "1.5K"},
		{"1048576 binary", 1048576, HumanSizeOpts{Binary: true}, "1M"},
		{"1073741824 binary", 1073741824, HumanSizeOpts{Binary: true}, "1G"},
		{"1099511627776 binary", 1099511627776, HumanSizeOpts{Binary: true}, "1T"},

		// AC1: boundary values in SI mode
		{"zero SI", 0, HumanSizeOpts{Binary: false}, "0"},
		{"999 SI", 999, HumanSizeOpts{Binary: false}, "999"},
		{"1000 SI", 1000, HumanSizeOpts{Binary: false}, "1.0kB"},
		{"1000000 SI", 1000000, HumanSizeOpts{Binary: false}, "1.0MB"},
		{"1500 SI", 1500, HumanSizeOpts{Binary: false}, "1.5kB"},
		{"1000000000 SI", 1000000000, HumanSizeOpts{Binary: false}, "1.0GB"},

		// AC4: specific values from acceptance criteria
		{"AC4 1536 binary", 1536, HumanSizeOpts{Binary: true}, "1.5K"},
		{"AC4 1000000 SI", 1000000, HumanSizeOpts{Binary: false}, "1.0MB"},
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
