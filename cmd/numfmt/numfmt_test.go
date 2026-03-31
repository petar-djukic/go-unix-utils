// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/numfmt.
// Traces: prd071-numfmt R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4,
// R3.1, R3.2, R3.3, R3.4, R4.1, R4.2, R4.3, R4.4.
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
		// R4.1, R4.2: invalid number exits 2 (default --invalid=abort)
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
		// R3.1: --field=2 converts only the second field
		{
			Name:  "field_2",
			Args:  []string{"--to=iec", "--field=2"},
			Stdin: []byte("name 1048576\n"),
		},
		// R3.1: --field=1 converts only the first field
		{
			Name:  "field_1",
			Args:  []string{"--to=iec", "--field=1"},
			Stdin: []byte("1048576 name\n"),
		},
		// R3.1: --field=2- converts from field 2 onwards
		{
			Name:  "field_2_onwards",
			Args:  []string{"--to=iec", "--field=2-"},
			Stdin: []byte("text 1024 1048576\n"),
		},
		// R3.1: --field=1,3 converts fields 1 and 3
		{
			Name:  "field_1_and_3",
			Args:  []string{"--to=iec", "--field=1,3"},
			Stdin: []byte("1024 text 1048576\n"),
		},
		// R3.2: --delimiter=: uses colon as field delimiter
		{
			Name:  "delimiter_colon",
			Args:  []string{"--to=iec", "--field=2", "--delimiter=:"},
			Stdin: []byte("name:1048576\n"),
		},
		// R3.2: -d , short form
		{
			Name:  "delimiter_short_comma",
			Args:  []string{"--to=iec", "--field=2", "-d", ","},
			Stdin: []byte("name,1048576\n"),
		},
		// R3.3: --header passes first line through unchanged
		{
			Name:  "header_default",
			Args:  []string{"--to=iec", "--header"},
			Stdin: []byte("size\n1048576\n"),
		},
		// R3.3: --header=2 passes first 2 lines through
		{
			Name:  "header_2",
			Args:  []string{"--to=iec", "--header=2"},
			Stdin: []byte("col1\ncol2\n1048576\n"),
		},
		// R3.3: --header combined with --field
		{
			Name:  "header_with_field",
			Args:  []string{"--to=iec", "--header", "--field=2"},
			Stdin: []byte("name size\nfoo 1048576\n"),
		},
		// R3.4: --from-unit multiplies input value
		{
			Name:  "from_unit_1024",
			Args:  []string{"--to=iec", "--from-unit=1024"},
			Stdin: []byte("1024\n"),
		},
		// R3.4: --to-unit divides output value
		{
			Name:  "to_unit_1000",
			Args:  []string{"--to-unit=1000"},
			Stdin: []byte("5000\n"),
		},
		// R3.4: --from-unit combined with --to
		{
			Name:  "from_unit_with_to_si",
			Args:  []string{"--to=si", "--from-unit=1000000"},
			Stdin: []byte("5\n"),
		},
		// R3.4: --to-unit combined with --to
		{
			Name:  "to_unit_with_to_si",
			Args:  []string{"--to=si", "--to-unit=1000"},
			Stdin: []byte("5000000\n"),
		},

		// --- R4.1, R4.2: exit code and --invalid mode tests ---

		// R4.1: exit 0 on successful conversion
		{
			Name:  "exit_0_success",
			Args:  []string{"--to=iec"},
			Stdin: []byte("1024\n"),
		},
		// R4.1: exit 0 on successful multi-line conversion
		{
			Name:  "exit_0_multiline_success",
			Args:  []string{"--to=si"},
			Stdin: []byte("1000\n2000\n3000\n"),
		},
		// R4.2: --invalid=fail prints error, continues, exits 2
		{
			Name:      "invalid_fail",
			Args:      []string{"--to=iec", "--invalid=fail"},
			Stdin:     []byte("abc\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// R4.2: --invalid=warn prints warning, continues, exits 0
		{
			Name:      "invalid_warn",
			Args:      []string{"--to=iec", "--invalid=warn"},
			Stdin:     []byte("abc\n"),
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// R4.2: --invalid=ignore silently passes through, exits 0
		{
			Name:  "invalid_ignore",
			Args:  []string{"--to=iec", "--invalid=ignore"},
			Stdin: []byte("abc\n"),
		},
		// R4.2: --invalid=fail with mixed valid/invalid lines
		{
			Name:      "invalid_fail_mixed",
			Args:      []string{"--to=iec", "--invalid=fail"},
			Stdin:     []byte("1024\nabc\n2048\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// R4.2: --invalid=warn with mixed valid/invalid lines
		{
			Name:      "invalid_warn_mixed",
			Args:      []string{"--to=iec", "--invalid=warn"},
			Stdin:     []byte("1024\nabc\n2048\n"),
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// R4.2: --invalid=ignore with mixed valid/invalid lines
		{
			Name:      "invalid_ignore_mixed",
			Args:      []string{"--to=iec", "--invalid=ignore"},
			Stdin:     []byte("1024\nabc\n2048\n"),
		},
		// R4.4: operand error case
		{
			Name:      "operand_invalid",
			Args:      []string{"--to=iec", "xyz"},
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
		// R4.4: unknown suffix error
		{
			Name:      "unknown_suffix_error",
			Args:      []string{"--from=si"},
			Stdin:     []byte("1Q\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeNonEmpty},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
