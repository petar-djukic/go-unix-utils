// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "testing"

// binary and si are the two canonical HumanSizeOpts values used throughout
// the tests and use case examples.
var (
	binary = HumanSizeOpts{Binary: true}
	si     = HumanSizeOpts{Binary: false}
)

// TestHumanSize covers prd003-format R3.1–R3.6, use case F7, and task AC4.
func TestHumanSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		bytes int64
		opts  HumanSizeOpts
		want  string
	}{
		// R3.4: zero always returns "0".
		{"zero binary", 0, binary, "0"},
		{"zero SI", 0, si, "0"},

		// Values below first threshold: plain integer, no suffix.
		{"sub-K binary", 512, binary, "512"},
		{"sub-kB SI", 999, si, "999"},
		{"one byte", 1, binary, "1"},

		// R3.3: non-integer values at the chosen unit use one decimal.
		// use case F7 example.
		{"1.5K binary", 1536, binary, "1.5K"},

		// Task AC4: powers of 1024 produce K/M/G-suffixed strings.
		{"1K binary", 1024, binary, "1.0K"},
		{"1M binary", 1048576, binary, "1.0M"},
		{"1G binary", 1073741824, binary, "1.0G"},

		// Task AC4 / use case F7: SI mode.
		{"1.0MB SI", 1000000, si, "1.0MB"},
		{"1.0kB SI", 1000, si, "1.0kB"},
		{"1.0GB SI", 1000000000, si, "1.0GB"},

		// Additional coverage.
		{"1.5M binary", 1572864, binary, "1.5M"},
		{"2K binary", 2048, binary, "2.0K"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, tc.opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, {Binary:%v}) = %q, want %q",
					tc.bytes, tc.opts.Binary, got, tc.want)
			}
		})
	}
}
