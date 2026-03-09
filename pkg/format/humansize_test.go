// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import (
	"testing"
)

func TestHumanSizeBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0"},
		{"one byte", 1, "1"},
		{"999 bytes", 999, "999"},
		{"1023 bytes", 1023, "1023"},
		{"1 KiB exactly", 1024, "1.0K"},
		{"1.5 KiB", 1536, "1.5K"},
		{"10 KiB", 10240, "10K"},
		{"15 KiB", 15360, "15K"},
		{"1 MiB exactly", 1048576, "1.0M"},
		{"1.5 MiB", 1572864, "1.5M"},
		{"10 MiB", 10485760, "10M"},
		{"1 GiB exactly", 1073741824, "1.0G"},
		{"1 TiB exactly", 1099511627776, "1.0T"},
		{"negative 1.5K", -1536, "-1.5K"},
		{"negative 1 byte", -1, "-1"},
		{"boundary 9.9K", 10138, "9.9K"},
	}

	opts := HumanSizeOpts{Binary: true}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, binary) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

func TestHumanSizeSI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{"zero", 0, "0"},
		{"one byte", 1, "1"},
		{"999 bytes", 999, "999"},
		{"1000 bytes", 1000, "1.0kB"},
		{"1500 bytes", 1500, "1.5kB"},
		{"15000 bytes", 15000, "15kB"},
		{"1 MB exactly", 1000000, "1.0MB"},
		{"1.5 MB", 1500000, "1.5MB"},
		{"10 MB", 10000000, "10MB"},
		{"1 GB exactly", 1000000000, "1.0GB"},
		{"1 TB exactly", 1000000000000, "1.0TB"},
		{"negative 1.5kB", -1500, "-1.5kB"},
	}

	opts := HumanSizeOpts{Binary: false}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, opts)
			if got != tc.want {
				t.Errorf("HumanSize(%d, SI) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

func TestHumanSizeEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		bytes  int64
		binary bool
		want   string
	}{
		{"zero binary", 0, true, "0"},
		{"zero SI", 0, false, "0"},
		{"negative zero is still zero", 0, true, "0"},
		{"max int64 binary", 9223372036854775807, true, "8.0E"},
		{"exact boundary 1024*1024", 1048576, true, "1.0M"},
		{"just below 10K binary", 10239, true, "10K"},
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
