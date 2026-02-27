// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "testing"

func TestHumanSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		opts  HumanSizeOpts
		want  string
	}{
		// AC5: explicit acceptance criteria values.
		{name: "binary_1536", bytes: 1536, opts: HumanSizeOpts{}, want: "1.5K"},
		{name: "si_1000000", bytes: 1000000, opts: HumanSizeOpts{SI: true}, want: "1.0MB"},
		{name: "zero", bytes: 0, opts: HumanSizeOpts{}, want: "0"},
		{name: "binary_1024", bytes: 1024, opts: HumanSizeOpts{}, want: "1.0K"},

		// Additional binary mode tests.
		{name: "binary_sub_k", bytes: 500, opts: HumanSizeOpts{}, want: "500"},
		{name: "binary_1", bytes: 1, opts: HumanSizeOpts{}, want: "1"},
		{name: "binary_1023", bytes: 1023, opts: HumanSizeOpts{}, want: "1023"},
		{name: "binary_1048576", bytes: 1048576, opts: HumanSizeOpts{}, want: "1.0M"},
		{name: "binary_1073741824", bytes: 1073741824, opts: HumanSizeOpts{}, want: "1.0G"},
		{name: "binary_1536000", bytes: 1572864, opts: HumanSizeOpts{}, want: "1.5M"}, // 1.5 * 1024^2

		// SI mode tests.
		{name: "si_zero", bytes: 0, opts: HumanSizeOpts{SI: true}, want: "0"},
		{name: "si_sub_k", bytes: 500, opts: HumanSizeOpts{SI: true}, want: "500"},
		{name: "si_1000", bytes: 1000, opts: HumanSizeOpts{SI: true}, want: "1.0kB"},
		{name: "si_1500", bytes: 1500, opts: HumanSizeOpts{SI: true}, want: "1.5kB"},
		{name: "si_1000000000", bytes: 1000000000, opts: HumanSizeOpts{SI: true}, want: "1.0GB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HumanSize(tt.bytes, tt.opts)
			if got != tt.want {
				t.Errorf("HumanSize(%d, %+v) = %q, want %q", tt.bytes, tt.opts, got, tt.want)
			}
		})
	}
}
