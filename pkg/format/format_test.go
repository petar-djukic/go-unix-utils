// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format

import "testing"

// TestHumanSize_LSContext verifies HumanSize for ls -h and ls -sh usage
// patterns. Implements prd003-format R3.5.
func TestHumanSize_LSContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		opts  HumanSizeOpts
		want  string
	}{
		// R3.5: ls -l -h file sizes in binary mode.
		{"ls_h_small_file", 743, HumanSizeOpts{Binary: true}, "743"},
		{"ls_h_4K_file", 4096, HumanSizeOpts{Binary: true}, "4K"},
		{"ls_h_large_file", 52428800, HumanSizeOpts{Binary: true}, "50M"},
		{"ls_h_fractional", 3584, HumanSizeOpts{Binary: true}, "3.5K"},

		// R3.5: ls -s block counts converted to bytes (1024-byte blocks).
		{"ls_s_8_blocks", 8 * 1024, HumanSizeOpts{Binary: true}, "8K"},
		{"ls_s_2048_blocks", 2048 * 1024, HumanSizeOpts{Binary: true}, "2M"},
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

// TestHumanSize_DUContext verifies HumanSize for du -h usage patterns.
// Implements prd003-format R3.6.
func TestHumanSize_DUContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		opts  HumanSizeOpts
		want  string
	}{
		// R3.6: du -h binary mode (default).
		{"du_h_small_dir", 4096, HumanSizeOpts{Binary: true}, "4K"},
		{"du_h_medium_dir", 157286400, HumanSizeOpts{Binary: true}, "150M"},
		{"du_h_large_dir", 5368709120, HumanSizeOpts{Binary: true}, "5G"},

		// R3.6: du --si mode (1000-based).
		{"du_si_small", 4000, HumanSizeOpts{Binary: false}, "4kB"},
		{"du_si_medium", 150000000, HumanSizeOpts{Binary: false}, "150MB"},
		{"du_si_large", 5000000000, HumanSizeOpts{Binary: false}, "5GB"},
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
