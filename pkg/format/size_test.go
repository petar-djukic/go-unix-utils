// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// size_test.go contains table-driven unit tests for HumanSize covering both
// binary (base 1024) and SI (base 1000) modes.
//
// Tests: prd003-format R3.1, R3.2, R3.3, R3.4.
package format

import "testing"

func TestHumanSize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		opts     HumanSizeOpts
		expected string
	}{
		// --- Zero bytes (R3.4) ---
		{name: "zero_binary", bytes: 0, opts: HumanSizeOpts{Binary: true}, expected: "0"},
		{name: "zero_si", bytes: 0, opts: HumanSizeOpts{Binary: false}, expected: "0"},

		// --- Binary mode (base 1024, suffixes K/M/G/T/P/E) (R3.1, R3.2) ---

		// Sub-unit values: raw integer, no suffix
		{name: "binary_1_byte", bytes: 1, opts: HumanSizeOpts{Binary: true}, expected: "1"},
		{name: "binary_512_bytes", bytes: 512, opts: HumanSizeOpts{Binary: true}, expected: "512"},
		{name: "binary_1023_bytes", bytes: 1023, opts: HumanSizeOpts{Binary: true}, expected: "1023"},

		// Exact boundary at K (1024)
		{name: "binary_exact_1K", bytes: 1024, opts: HumanSizeOpts{Binary: true}, expected: "1.0K"},

		// One decimal place for values under 10 at given unit (R3.3)
		{name: "binary_1536_bytes_1.5K", bytes: 1536, opts: HumanSizeOpts{Binary: true}, expected: "1.5K"},
		{name: "binary_2048_bytes_2.0K", bytes: 2048, opts: HumanSizeOpts{Binary: true}, expected: "2.0K"},
		{name: "binary_5120_bytes_5.0K", bytes: 5120, opts: HumanSizeOpts{Binary: true}, expected: "5.0K"},
		{name: "binary_9216_bytes_9.0K", bytes: 9216, opts: HumanSizeOpts{Binary: true}, expected: "9.0K"},

		// Integer display for values >= 10 at given unit
		{name: "binary_10240_bytes_10K", bytes: 10240, opts: HumanSizeOpts{Binary: true}, expected: "10K"},
		{name: "binary_102400_bytes_100K", bytes: 102400, opts: HumanSizeOpts{Binary: true}, expected: "100K"},

		// M boundary (1024*1024 = 1048576)
		{name: "binary_exact_1M", bytes: 1048576, opts: HumanSizeOpts{Binary: true}, expected: "1.0M"},
		{name: "binary_1.5M", bytes: 1572864, opts: HumanSizeOpts{Binary: true}, expected: "1.5M"},
		{name: "binary_10M", bytes: 10485760, opts: HumanSizeOpts{Binary: true}, expected: "10M"},

		// G boundary (1024^3 = 1073741824)
		{name: "binary_exact_1G", bytes: 1073741824, opts: HumanSizeOpts{Binary: true}, expected: "1.0G"},
		{name: "binary_1.5G", bytes: 1610612736, opts: HumanSizeOpts{Binary: true}, expected: "1.5G"},

		// T boundary (1024^4)
		{name: "binary_exact_1T", bytes: 1099511627776, opts: HumanSizeOpts{Binary: true}, expected: "1.0T"},

		// P boundary (1024^5)
		{name: "binary_exact_1P", bytes: 1125899906842624, opts: HumanSizeOpts{Binary: true}, expected: "1.0P"},

		// E boundary (1024^6)
		{name: "binary_exact_1E", bytes: 1152921504606846976, opts: HumanSizeOpts{Binary: true}, expected: "1.0E"},

		// --- SI mode (base 1000, suffixes kB/MB/GB/TB) (R3.1, R3.2) ---

		// Sub-unit values
		{name: "si_1_byte", bytes: 1, opts: HumanSizeOpts{Binary: false}, expected: "1"},
		{name: "si_999_bytes", bytes: 999, opts: HumanSizeOpts{Binary: false}, expected: "999"},

		// kB boundary (1000)
		{name: "si_exact_1kB", bytes: 1000, opts: HumanSizeOpts{Binary: false}, expected: "1.0kB"},
		{name: "si_1500_bytes_1.5kB", bytes: 1500, opts: HumanSizeOpts{Binary: false}, expected: "1.5kB"},
		{name: "si_10000_bytes_10kB", bytes: 10000, opts: HumanSizeOpts{Binary: false}, expected: "10kB"},

		// MB boundary (1000000)
		{name: "si_exact_1MB", bytes: 1000000, opts: HumanSizeOpts{Binary: false}, expected: "1.0MB"},
		{name: "si_1500000_bytes_1.5MB", bytes: 1500000, opts: HumanSizeOpts{Binary: false}, expected: "1.5MB"},

		// GB boundary (1000000000)
		{name: "si_exact_1GB", bytes: 1000000000, opts: HumanSizeOpts{Binary: false}, expected: "1.0GB"},

		// TB boundary (1000000000000)
		{name: "si_exact_1TB", bytes: 1000000000000, opts: HumanSizeOpts{Binary: false}, expected: "1.0TB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HumanSize(tt.bytes, tt.opts)
			if got != tt.expected {
				t.Errorf("HumanSize(%d, {Binary: %v}) = %q, want %q",
					tt.bytes, tt.opts.Binary, got, tt.expected)
			}
		})
	}
}
