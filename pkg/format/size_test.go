// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "testing"

func TestHumanSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		bytes  int64
		binary bool
		want   string
	}{
		// R3.4: zero
		{"zero binary", 0, true, "0"},
		{"zero SI", 0, false, "0"},

		// AC1: boundary values, binary mode
		{"999 binary", 999, true, "999"},
		{"1023 binary", 1023, true, "1023"},
		{"1024 binary", 1024, true, "1.0K"},
		{"1536 binary (AC4)", 1536, true, "1.5K"},
		{"1048576 binary", 1048576, true, "1.0M"},
		{"10240 binary", 10240, true, "10.0K"},
		{"102400 binary", 102400, true, "100K"},
		{"1073741824 binary", 1073741824, true, "1.0G"},

		// AC1: boundary values, SI mode
		{"999 SI", 999, false, "999"},
		{"1000 SI", 1000, false, "1.0kB"},
		{"1000000 SI (AC4)", 1000000, false, "1.0MB"},
		{"1500 SI", 1500, false, "1.5kB"},
		{"100000 SI", 100000, false, "100kB"},
		{"1000000000 SI", 1000000000, false, "1.0GB"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, HumanSizeOpts{Binary: tc.binary})
			if got != tc.want {
				t.Errorf("HumanSize(%d, binary=%v) = %q, want %q", tc.bytes, tc.binary, got, tc.want)
			}
		})
	}
}
