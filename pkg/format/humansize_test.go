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
		// R3.4: zero always returns "0".
		{"zero binary", 0, HumanSizeOpts{Binary: true}, "0"},
		{"zero SI", 0, HumanSizeOpts{Binary: false}, "0"},

		// Binary mode — below 1 KiB: bare integer.
		{"1 byte binary", 1, HumanSizeOpts{Binary: true}, "1"},
		{"512 bytes binary", 512, HumanSizeOpts{Binary: true}, "512"},
		{"1023 bytes binary", 1023, HumanSizeOpts{Binary: true}, "1023"},

		// Binary mode — K range.
		{"1024 bytes = 1.0K", 1024, HumanSizeOpts{Binary: true}, "1.0K"},
		{"1536 bytes = 1.5K (AC4)", 1536, HumanSizeOpts{Binary: true}, "1.5K"},
		{"2048 bytes = 2.0K", 2048, HumanSizeOpts{Binary: true}, "2.0K"},

		// Binary mode — M range.
		{"1 MiB = 1.0M", 1024 * 1024, HumanSizeOpts{Binary: true}, "1.0M"},
		{"1.5 MiB = 1.5M", 1536 * 1024, HumanSizeOpts{Binary: true}, "1.5M"},

		// Binary mode — G range.
		{"1 GiB = 1.0G", 1024 * 1024 * 1024, HumanSizeOpts{Binary: true}, "1.0G"},

		// Binary mode — T range.
		{"1 TiB = 1.0T", 1024 * 1024 * 1024 * 1024, HumanSizeOpts{Binary: true}, "1.0T"},

		// SI mode — below 1 kB: bare integer.
		{"1 byte SI", 1, HumanSizeOpts{Binary: false}, "1"},
		{"999 bytes SI", 999, HumanSizeOpts{Binary: false}, "999"},

		// SI mode — kB range.
		{"1000 bytes = 1.0kB", 1000, HumanSizeOpts{Binary: false}, "1.0kB"},
		{"1500 bytes = 1.5kB", 1500, HumanSizeOpts{Binary: false}, "1.5kB"},

		// SI mode — MB range.
		{"1000000 bytes = 1.0MB (AC4)", 1000000, HumanSizeOpts{Binary: false}, "1.0MB"},
		{"1500000 bytes = 1.5MB", 1500000, HumanSizeOpts{Binary: false}, "1.5MB"},

		// SI mode — GB range.
		{"1000000000 bytes = 1.0GB", 1000000000, HumanSizeOpts{Binary: false}, "1.0GB"},

		// SI mode — TB range.
		{"1000000000000 bytes = 1.0TB", 1000000000000, HumanSizeOpts{Binary: false}, "1.0TB"},
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
