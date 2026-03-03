// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package format_test

import (
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/format"
)

// TestHumanSize verifies byte-to-human conversion for binary (1024-based)
// and SI (1000-based) modes. (prd003-format R3.1–R3.6; AC4, use case F7, S6)
func TestHumanSize(t *testing.T) {
	binary := format.HumanSizeOpts{Binary: true}
	si := format.HumanSizeOpts{Binary: false}

	tests := []struct {
		name     string
		bytes    int64
		opts     format.HumanSizeOpts
		expected string
	}{
		// Zero returns "0" regardless of mode (prd003-format R3.4; AC4).
		{"zero binary", 0, binary, "0"},
		{"zero SI", 0, si, "0"},

		// Values below the first unit boundary: returned as a plain integer.
		{"below 1K binary", 1000, binary, "1000"},
		{"below 1kB SI", 999, si, "999"},
		{"one byte binary", 1, binary, "1"},

		// Binary mode: 1024-based with K/M/G/T suffixes (prd003-format R3.2).
		// AC4: HumanSize(1536, binary) = "1.5K"
		{"1536 binary = 1.5K", 1536, binary, "1.5K"},
		{"1024 binary = 1.0K", 1024, binary, "1.0K"},
		{"1048576 binary = 1.0M", 1048576, binary, "1.0M"},
		{"1073741824 binary = 1.0G", 1073741824, binary, "1.0G"},
		{"1610612736 binary = 1.5G", 1610612736, binary, "1.5G"},

		// SI mode: 1000-based with kB/MB/GB suffixes (prd003-format R3.2).
		// AC4: HumanSize(1000000, SI) = "1.0MB"
		{"1000000 SI = 1.0MB", 1000000, si, "1.0MB"},
		{"1000 SI = 1.0kB", 1000, si, "1.0kB"},
		{"1500000 SI = 1.5MB", 1500000, si, "1.5MB"},
		{"1000000000 SI = 1.0GB", 1000000000, si, "1.0GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := format.HumanSize(tt.bytes, tt.opts)
			if got != tt.expected {
				t.Errorf("HumanSize(%d, Binary=%v) = %q, want %q",
					tt.bytes, tt.opts.Binary, got, tt.expected)
			}
		})
	}
}
