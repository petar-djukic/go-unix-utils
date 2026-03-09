// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"testing"
)

func TestHumanSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		opts  HumanSizeOpts
		want  string
	}{
		// R3.4: zero returns "0".
		{"zero binary", 0, HumanSizeOpts{Binary: true}, "0"},
		{"zero SI", 0, HumanSizeOpts{Binary: false}, "0"},
		{"zero default", 0, HumanSizeOpts{}, "0"},

		// Small values below threshold (raw bytes).
		{"one byte binary", 1, HumanSizeOpts{Binary: true}, "1"},
		{"999 bytes binary", 999, HumanSizeOpts{Binary: true}, "999"},
		{"999 bytes SI", 999, HumanSizeOpts{Binary: false}, "999"},

		// Binary mode (base 1024) — R3.2, R3.3.
		{"1024 binary", 1024, HumanSizeOpts{Binary: true}, "1.0K"},
		{"1536 binary", 1536, HumanSizeOpts{Binary: true}, "1.5K"},
		{"1048576 binary", 1048576, HumanSizeOpts{Binary: true}, "1.0M"},
		{"1073741824 binary", 1073741824, HumanSizeOpts{Binary: true}, "1.0G"},
		{"1099511627776 binary", 1099511627776, HumanSizeOpts{Binary: true}, "1.0T"},
		{"1125899906842624 binary", 1125899906842624, HumanSizeOpts{Binary: true}, "1.0P"},
		{"1152921504606846976 binary", 1152921504606846976, HumanSizeOpts{Binary: true}, "1.0E"},

		// SI mode (base 1000) — R3.2.
		{"1000 SI", 1000, HumanSizeOpts{Binary: false}, "1.0kB"},
		{"1500 SI", 1500, HumanSizeOpts{Binary: false}, "1.5kB"},
		{"1000000 SI", 1000000, HumanSizeOpts{Binary: false}, "1.0MB"},
		{"1000000000 SI", 1000000000, HumanSizeOpts{Binary: false}, "1.0GB"},
		{"1000000000000 SI", 1000000000000, HumanSizeOpts{Binary: false}, "1.0TB"},

		// Ceiling rounding (GNU coreutils behavior).
		{"1025 binary ceiling", 1025, HumanSizeOpts{Binary: true}, "1.1K"},
		{"1126 binary ceiling", 1126, HumanSizeOpts{Binary: true}, "1.1K"},

		// Values that round to whole at unit.
		{"2048 binary", 2048, HumanSizeOpts{Binary: true}, "2.0K"},
		{"10240 binary", 10240, HumanSizeOpts{Binary: true}, "10.0K"},

		// Default opts (Binary: false is zero value).
		{"default opts is SI", 1000, HumanSizeOpts{}, "1.0kB"},

		// R3.5: ls -s block counts converted to human-readable.
		// ls -s shows allocated 1024-byte blocks; ls -sh converts via HumanSize.
		{"ls -s 4 blocks", 4 * 1024, HumanSizeOpts{Binary: true}, "4.0K"},
		{"ls -s 8 blocks", 8 * 1024, HumanSizeOpts{Binary: true}, "8.0K"},
		{"ls -s 128 blocks", 128 * 1024, HumanSizeOpts{Binary: true}, "128.0K"},
		{"ls -s large file blocks", 4096 * 1024, HumanSizeOpts{Binary: true}, "4.0M"},

		// R3.6: du -h directory sizes as human-readable strings.
		// du reports byte counts; du -h converts via HumanSize with same binary/SI distinction.
		{"du -h small dir", 24576, HumanSizeOpts{Binary: true}, "24.0K"},
		{"du -h medium dir", 5242880, HumanSizeOpts{Binary: true}, "5.0M"},
		{"du -h large dir", 2147483648, HumanSizeOpts{Binary: true}, "2.0G"},
		{"du -h SI mode", 5000000, HumanSizeOpts{Binary: false}, "5.0MB"},
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
