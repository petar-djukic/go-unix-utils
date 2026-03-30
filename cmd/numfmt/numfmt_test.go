// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/numfmt.
// Traces: prd071-numfmt R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeNonEmpty replaces any non-empty output with a fixed marker.
// Used for stderr where message format differs between Go and GNU binaries.
func normalizeNonEmpty(b []byte) []byte {
	if len(b) > 0 {
		return []byte("ERROR\n")
	}
	return b
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnumfmt")
	if err != nil {
		t.Skip("reference binary gnumfmt not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.1: --to=iec converts raw number to human-readable IEC suffix
		{
			Name:  "to_iec_1M",
			Args:  []string{"--to=iec"},
			Stdin: []byte("1048576\n"),
		},
		{
			Name:  "to_iec_1K",
			Args:  []string{"--to=iec"},
			Stdin: []byte("1024\n"),
		},
		{
			Name:  "to_iec_small",
			Args:  []string{"--to=iec"},
			Stdin: []byte("500\n"),
		},
		{
			Name:  "to_iec_zero",
			Args:  []string{"--to=iec"},
			Stdin: []byte("0\n"),
		},
		// R1.1: --to=si converts raw number to human-readable SI suffix
		{
			Name:  "to_si_1M",
			Args:  []string{"--to=si"},
			Stdin: []byte("1000000\n"),
		},
		{
			Name:  "to_si_1500",
			Args:  []string{"--to=si"},
			Stdin: []byte("1500\n"),
		},
		// R1.1: --to=iec-i uses Ki/Mi/Gi suffixes
		{
			Name:  "to_iec_i_1M",
			Args:  []string{"--to=iec-i"},
			Stdin: []byte("1048576\n"),
		},
		// R1.2: --from=si parses SI suffixes to raw number
		{
			Name:  "from_si_1K",
			Args:  []string{"--from=si"},
			Stdin: []byte("1K\n"),
		},
		{
			Name:  "from_si_1M",
			Args:  []string{"--from=si"},
			Stdin: []byte("1M\n"),
		},
		{
			Name:  "from_si_1point5K",
			Args:  []string{"--from=si"},
			Stdin: []byte("1.5K\n"),
		},
		// R1.2: --from=iec parses IEC suffixes
		{
			Name:  "from_iec_1K",
			Args:  []string{"--from=iec"},
			Stdin: []byte("1K\n"),
		},
		// R1.2: --from=iec-i parses Ki/Mi suffixes
		{
			Name:  "from_iec_i_1Ki",
			Args:  []string{"--from=iec-i"},
			Stdin: []byte("1Ki\n"),
		},
		// R1.2 + R1.1: --from combined with --to
		{
			Name:  "from_si_to_iec",
			Args:  []string{"--from=si", "--to=iec"},
			Stdin: []byte("1M\n"),
		},
		// R1.3: passthrough without --from or --to
		{
			Name:  "passthrough_integer",
			Stdin: []byte("42\n"),
		},
		{
			Name:  "passthrough_float",
			Stdin: []byte("3.14\n"),
		},
		// R1.4: process command-line operands directly
		{
			Name: "operand_to_iec",
			Args: []string{"--to=iec", "1048576"},
		},
		{
			Name: "operand_passthrough",
			Args: []string{"42"},
		},
		{
			Name: "operand_multiple",
			Args: []string{"--to=iec", "1024", "1048576"},
		},
		// R1.4: stdin with multiple lines
		{
			Name:  "stdin_multiline",
			Args:  []string{"--to=iec"},
			Stdin: []byte("1024\n1048576\n1073741824\n"),
		},
		// Error: invalid number exits 2
		{
			Name:      "invalid_number",
			Args:      []string{"--to=iec"},
			Stdin:     []byte("abc\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// R1.1: default from-zero rounding matches GNU
		{
			Name:  "default_rounding_1111",
			Args:  []string{"--to=si"},
			Stdin: []byte("1111\n"),
		},
		// R2.1: --format with width (right-align)
		{
			Name:  "format_width_right",
			Args:  []string{"--to=si", "--format=%10f"},
			Stdin: []byte("1500\n"),
		},
		// R2.1: --format with precision
		{
			Name:  "format_precision",
			Args:  []string{"--to=si", "--format=%.2f"},
			Stdin: []byte("1500\n"),
		},
		// R2.1: --format with left alignment
		{
			Name:  "format_left_align",
			Args:  []string{"--to=si", "--format=%-10f"},
			Stdin: []byte("1500\n"),
		},
		// R2.1: --format with width and precision
		{
			Name:  "format_width_precision",
			Args:  []string{"--to=si", "--format=%10.2f"},
			Stdin: []byte("1500\n"),
		},
		// R2.2: --padding right-align
		{
			Name:  "padding_right",
			Args:  []string{"--to=iec", "--padding=10"},
			Stdin: []byte("1048576\n"),
		},
		// R2.2: --padding left-align
		{
			Name:  "padding_left",
			Args:  []string{"--to=iec", "--padding=-10"},
			Stdin: []byte("1048576\n"),
		},
		// R2.3: --round=up
		{
			Name:  "round_up",
			Args:  []string{"--to=si", "--round=up"},
			Stdin: []byte("1444\n"),
		},
		// R2.3: --round=down
		{
			Name:  "round_down",
			Args:  []string{"--to=si", "--round=down"},
			Stdin: []byte("1555\n"),
		},
		// R2.3: --round=towards-zero
		{
			Name:  "round_towards_zero",
			Args:  []string{"--to=si", "--round=towards-zero"},
			Stdin: []byte("1555\n"),
		},
		// R2.3: --round=from-zero
		{
			Name:  "round_from_zero",
			Args:  []string{"--to=si", "--round=from-zero"},
			Stdin: []byte("1444\n"),
		},
		// R2.3: --round=nearest
		{
			Name:  "round_nearest",
			Args:  []string{"--to=si", "--round=nearest"},
			Stdin: []byte("1444\n"),
		},
		// R2.4: --suffix appended to output
		{
			Name:  "suffix_append",
			Args:  []string{"--to=si", "--suffix=B"},
			Stdin: []byte("1500\n"),
		},
		// R2.4: --suffix stripped from input and re-appended
		{
			Name:  "suffix_from_si",
			Args:  []string{"--from=si", "--suffix=B"},
			Stdin: []byte("1.5kB\n"),
		},
		// R2.4: --suffix with --from and --to
		{
			Name:  "suffix_from_to",
			Args:  []string{"--from=si", "--to=iec", "--suffix=B"},
			Stdin: []byte("1MB\n"),
		},
		// R2.2 + R2.1: --padding overrides --format width
		{
			Name:  "padding_with_format",
			Args:  []string{"--to=si", "--format=%.2f", "--padding=15"},
			Stdin: []byte("1500\n"),
		},
		// R2.3: rounding with magnitude change
		{
			Name:  "round_magnitude_change",
			Args:  []string{"--to=si"},
			Stdin: []byte("9950\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
