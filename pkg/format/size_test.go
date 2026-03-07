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
		// AC1: SI units (Binary=false).
		{"si zero", 0, HumanSizeOpts{Binary: false}, "0"},
		{"si sub-threshold", 999, HumanSizeOpts{Binary: false}, "999"},
		{"si 1K", 1000, HumanSizeOpts{Binary: false}, "1.0K"},
		{"si 1.5M", 1500000, HumanSizeOpts{Binary: false}, "1.5M"},
		{"si 1G", 1000000000, HumanSizeOpts{Binary: false}, "1.0G"},

		// AC2: binary units (Binary=true).
		{"binary zero", 0, HumanSizeOpts{Binary: true}, "0"},
		{"binary sub-threshold", 512, HumanSizeOpts{Binary: true}, "512"},
		{"binary 1Ki", 1024, HumanSizeOpts{Binary: true}, "1.0Ki"},
		{"binary 1Mi", 1048576, HumanSizeOpts{Binary: true}, "1.0Mi"},
		{"binary 1.5Ki", 1536, HumanSizeOpts{Binary: true}, "1.5Ki"},
		{"binary 1Gi", 1073741824, HumanSizeOpts{Binary: true}, "1.0Gi"},
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
