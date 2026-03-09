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

// TestHumanSizeLsContext verifies HumanSize for ls -h file size output. R3.5.
func TestHumanSizeLsContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		// ls -h file sizes (binary mode, base 1024). R3.5.
		{"directory block size 4096", 4096, "4.0K"},
		{"small file 256 bytes", 256, "256"},
		{"medium file 65536 bytes", 65536, "64K"},
		{"large file 10 MiB", 10485760, "10M"},
		{"photo file 3.5 MiB", 3670016, "3.5M"},
		{"video file 1.2 GiB", 1288490189, "1.2G"},
		// ls -s block counts are also passed through HumanSize. R3.5.
		{"ls -s 8 blocks * 512 = 4096", 4096, "4.0K"},
		{"ls -s 1 block * 512 = 512", 512, "512"},
		{"ls -s large block count", 524288, "512K"},
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

// TestHumanSizeDuContext verifies HumanSize for du -h directory size output. R3.6.
func TestHumanSizeDuContext(t *testing.T) {
	t.Parallel()

	t.Run("du -h binary", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name  string
			bytes int64
			want  string
		}{
			// du -h directory sizes (binary mode). R3.6.
			{"small directory 4K", 4096, "4.0K"},
			{"node_modules 256 MiB", 268435456, "256M"},
			{"home directory 50 GiB", 53687091200, "50G"},
			{"disk usage 1.5 TiB", 1649267441664, "1.5T"},
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
	})

	t.Run("du --si", func(t *testing.T) {
		t.Parallel()
		// du uses the same binary/SI distinction as ls -h. R3.6.
		tests := []struct {
			name  string
			bytes int64
			want  string
		}{
			{"small directory 4000", 4000, "4.0kB"},
			{"medium directory 250 MB", 250000000, "250MB"},
			{"large directory 1.5 GB", 1500000000, "1.5GB"},
			{"very large 2 TB", 2000000000000, "2.0TB"},
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
	})
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
