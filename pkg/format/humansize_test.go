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
		// R1.4: zero
		{"zero SI", 0, false, "0B"},
		{"zero IEC", 0, true, "0B"},

		// R1.2/R1.3: SI mode (base 1000)
		{"999B SI", 999, false, "999B"},
		{"1000 SI", 1000, false, "1K"},
		{"1500 SI", 1500, false, "1.5K"},
		{"1000000 SI", 1000000, false, "1M"},
		{"1500000 SI", 1500000, false, "1.5M"},
		{"1000000000 SI", 1000000000, false, "1G"},
		{"1000000000000 SI", 1000000000000, false, "1T"},
		{"1000000000000000 SI", 1000000000000000, false, "1P"},
		{"1000000000000000000 SI", 1000000000000000000, false, "1E"},

		// R1.2/R1.3: IEC/binary mode (base 1024)
		{"1023B IEC", 1023, true, "1023B"},
		{"1024 IEC", 1024, true, "1Ki"},
		{"1536 IEC", 1536, true, "1.5Ki"},
		{"1048576 IEC", 1048576, true, "1Mi"},
		{"1073741824 IEC", 1073741824, true, "1Gi"},
		{"1099511627776 IEC", 1099511627776, true, "1Ti"},

		// R1.4: negative values
		{"negative SI", -1500, false, "-1.5K"},
		{"negative IEC", -1536, true, "-1.5Ki"},
		{"negative small", -500, false, "-500B"},

		// R1.3: no trailing zeros
		{"exact 2K SI", 2000, false, "2K"},
		{"exact 2Ki IEC", 2048, true, "2Ki"},

		// R1.3: values that would have trailing zero after decimal
		{"1.0K displays as 1K", 1000, false, "1K"},
		{"10K SI", 10000, false, "10K"},
		{"1.5M SI", 1500000, false, "1.5M"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, HumanSizeOpts{Binary: tc.binary})
			if got != tc.want {
				t.Errorf("HumanSize(%d, {Binary: %v}) = %q, want %q", tc.bytes, tc.binary, got, tc.want)
			}
		})
	}
}

// TestHumanSizeLsContext verifies HumanSize works for ls -h use cases (R3.5).
// ls -h shows file sizes in -l output and block counts in -s output using
// binary (base-1024) formatting.
func TestHumanSizeLsContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		// R3.5: ls -h file sizes in -l output (binary mode).
		{"small file 100B", 100, "100B"},
		{"4K block", 4096, "4Ki"},
		{"typical source file", 15360, "15Ki"},
		{"medium file 1.5Mi", 1572864, "1.5Mi"},
		{"large file 2Gi", 2147483648, "2Gi"},

		// R3.5: ls -s block counts (512-byte blocks displayed as human-readable).
		{"8 blocks (4Ki)", 4096, "4Ki"},
		{"single block 512B", 512, "512B"},
		{"many blocks 100Mi", 104857600, "100Mi"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, HumanSizeOpts{Binary: true})
			if got != tc.want {
				t.Errorf("HumanSize(%d, {Binary: true}) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

// TestHumanSizeDuContext verifies HumanSize works for du -h use cases (R3.6).
// du -h outputs directory sizes as human-readable strings using the same
// binary/SI distinction as ls -h.
func TestHumanSizeDuContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		bytes  int64
		binary bool
		want   string
	}{
		// R3.6: du -h directory sizes (binary mode, same as ls -h).
		{"empty dir", 0, true, "0B"},
		{"small dir 4Ki", 4096, true, "4Ki"},
		{"medium dir 256Ki", 262144, true, "256Ki"},
		{"large dir 1.5Gi", 1610612736, true, "1.5Gi"},
		{"very large dir 2Ti", 2199023255552, true, "2Ti"},

		// R3.6: du --si directory sizes (SI mode).
		{"small dir SI", 4000, false, "4K"},
		{"medium dir SI", 250000, false, "250K"},
		{"large dir SI", 1500000000, false, "1.5G"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := HumanSize(tc.bytes, HumanSizeOpts{Binary: tc.binary})
			if got != tc.want {
				t.Errorf("HumanSize(%d, {Binary: %v}) = %q, want %q", tc.bytes, tc.binary, got, tc.want)
			}
		})
	}
}
