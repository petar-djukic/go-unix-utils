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
