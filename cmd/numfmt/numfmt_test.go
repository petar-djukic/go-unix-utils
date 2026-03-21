// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/numfmt against gnumfmt reference binary.
// Tests prd071-numfmt R1.1–R1.4, R2.1–R2.4, R3.1–R3.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnumfmt")
	if err != nil {
		t.Skipf("reference binary gnumfmt not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R1.3/R1.4: pass-through with no conversion flags.
		{Name: "passthrough_integer", Stdin: []byte("42\n")},
		{Name: "passthrough_negative", Stdin: []byte("-100\n")},

		// R1.1: --to=si output scaling.
		{Name: "to_si_thousand", Args: []string{"--to=si"}, Stdin: []byte("1000\n")},
		{Name: "to_si_million", Args: []string{"--to=si"}, Stdin: []byte("1500000\n")},
		{Name: "to_si_small", Args: []string{"--to=si"}, Stdin: []byte("500\n")},
		{Name: "to_si_zero", Args: []string{"--to=si"}, Stdin: []byte("0\n")},
		{Name: "to_si_negative", Args: []string{"--to=si"}, Stdin: []byte("-1000\n")},

		// R1.1: --to=iec output scaling.
		{Name: "to_iec_mebibyte", Args: []string{"--to=iec"}, Stdin: []byte("1048576\n")},
		{Name: "to_iec_gibibyte", Args: []string{"--to=iec"}, Stdin: []byte("1073741824\n")},

		// R1.1: --to=iec-i output scaling.
		{Name: "to_iec_i_mebibyte", Args: []string{"--to=iec-i"}, Stdin: []byte("1048576\n")},

		// R1.2: --from=si input scaling.
		{Name: "from_si_K", Args: []string{"--from=si"}, Stdin: []byte("1K\n")},
		{Name: "from_si_M", Args: []string{"--from=si"}, Stdin: []byte("5M\n")},
		{Name: "from_si_decimal", Args: []string{"--from=si"}, Stdin: []byte("1.5K\n")},

		// R1.2: --from=iec input scaling.
		{Name: "from_iec_K", Args: []string{"--from=iec"}, Stdin: []byte("1K\n")},

		// R1.2: --from=iec-i input scaling.
		{Name: "from_iec_i_Ki", Args: []string{"--from=iec-i"}, Stdin: []byte("1Ki\n")},

		// R1.2: --from=auto input scaling.
		{Name: "from_auto_K", Args: []string{"--from=auto"}, Stdin: []byte("1K\n")},
		{Name: "from_auto_Ki", Args: []string{"--from=auto"}, Stdin: []byte("1Ki\n")},

		// R1.4: operand mode (numbers as arguments).
		{Name: "operand_to_si", Args: []string{"--to=si", "1000"}},
		{Name: "multiple_operands", Args: []string{"--to=si", "1000", "2000000"}},

		// R1.4: multiple stdin lines.
		{
			Name:  "multiple_lines",
			Args:  []string{"--to=si"},
			Stdin: []byte("1000\n2000000\n"),
		},

		// R1.1/R1.2: combined --from and --to.
		{
			Name:  "from_iec_to_si",
			Args:  []string{"--from=iec", "--to=si"},
			Stdin: []byte("1K\n"),
		},

		// R2.1: --format with width.
		{
			Name:  "format_width_10",
			Args:  []string{"--to=si", "--format=%10f"},
			Stdin: []byte("1500000\n"),
		},
		// R2.1: --format with precision.
		{
			Name:  "format_precision_2",
			Args:  []string{"--to=si", "--format=%.2f"},
			Stdin: []byte("1500000\n"),
		},
		// R2.1: --format with width and precision.
		{
			Name:  "format_width_precision",
			Args:  []string{"--to=si", "--format=%10.2f"},
			Stdin: []byte("1500000\n"),
		},
		// R2.1: --format with left-alignment.
		{
			Name:  "format_left_align",
			Args:  []string{"--to=si", "--format=%-10f"},
			Stdin: []byte("1500000\n"),
		},
		// R2.1: --format with no --to (raw number formatting).
		{
			Name:  "format_raw_precision",
			Args:  []string{"--format=%.2f"},
			Stdin: []byte("3.14159\n"),
		},

		// R2.2: --padding positive (right-align).
		{
			Name:  "padding_right_align",
			Args:  []string{"--to=si", "--padding=10"},
			Stdin: []byte("1000\n"),
		},
		// R2.2: --padding negative (left-align).
		{
			Name:  "padding_left_align",
			Args:  []string{"--to=si", "--padding=-10"},
			Stdin: []byte("1000\n"),
		},
		// R2.2: --padding with no --to.
		{
			Name:  "padding_raw",
			Args:  []string{"--padding=10"},
			Stdin: []byte("42\n"),
		},

		// R2.3: --round=up (ceiling).
		{
			Name:  "round_up",
			Args:  []string{"--to=si", "--round=up"},
			Stdin: []byte("1234\n"),
		},
		// R2.3: --round=down (floor).
		{
			Name:  "round_down",
			Args:  []string{"--to=si", "--round=down"},
			Stdin: []byte("1234\n"),
		},
		// R2.3: --round=nearest.
		{
			Name:  "round_nearest",
			Args:  []string{"--to=si", "--round=nearest"},
			Stdin: []byte("1500\n"),
		},
		// R2.3: --round=towards-zero.
		{
			Name:  "round_towards_zero",
			Args:  []string{"--to=si", "--round=towards-zero"},
			Stdin: []byte("1900\n"),
		},
		// R2.3: --round=from-zero (explicit default).
		{
			Name:  "round_from_zero",
			Args:  []string{"--to=si", "--round=from-zero"},
			Stdin: []byte("1234\n"),
		},

		// R2.4: --suffix basic.
		{
			Name:  "suffix_basic",
			Args:  []string{"--to=si", "--suffix=B"},
			Stdin: []byte("1000\n"),
		},
		// R2.4: --suffix without --to.
		{
			Name:  "suffix_no_to",
			Args:  []string{"--suffix=X"},
			Stdin: []byte("42\n"),
		},

		// R2.1+R2.4: --format and --suffix combined.
		{
			Name:  "format_and_suffix",
			Args:  []string{"--to=si", "--format=%.2f", "--suffix=B"},
			Stdin: []byte("1500000\n"),
		},
		// R2.2+R2.4: --padding and --suffix combined.
		{
			Name:  "padding_and_suffix",
			Args:  []string{"--to=si", "--padding=15", "--suffix=B"},
			Stdin: []byte("1000\n"),
		},
		// R2.3: --round with --format precision.
		{
			Name:  "round_up_format",
			Args:  []string{"--format=%.0f", "--round=up"},
			Stdin: []byte("1.3\n"),
		},
		// R2.3: --round=down with --format precision.
		{
			Name:  "round_down_format",
			Args:  []string{"--format=%.0f", "--round=down"},
			Stdin: []byte("1.7\n"),
		},

		// R3.1: --field=N converts only specified field.
		{
			Name:  "field_single",
			Args:  []string{"--to=si", "--field=2"},
			Stdin: []byte("name 1000\n"),
		},
		// R3.1: --field with range.
		{
			Name:  "field_range",
			Args:  []string{"--to=si", "--field=2-3"},
			Stdin: []byte("x 1000 2000000 y\n"),
		},
		// R3.1: --field with comma-separated values.
		{
			Name:  "field_multiple",
			Args:  []string{"--to=si", "--field=1,3"},
			Stdin: []byte("1000 text 2000000\n"),
		},
		// R3.1: --field passthrough for unselected fields.
		{
			Name:  "field_passthrough",
			Args:  []string{"--to=iec", "--field=2"},
			Stdin: []byte("keep 1048576 also_keep\n"),
		},

		// R3.2: --delimiter with comma.
		{
			Name:  "delimiter_comma",
			Args:  []string{"--to=si", "--field=2", "--delimiter=,"},
			Stdin: []byte("name,1000\n"),
		},
		// R3.2: -d short form with colon.
		{
			Name:  "delimiter_short_colon",
			Args:  []string{"--to=iec", "--field=2", "-d", ":"},
			Stdin: []byte("size:1048576\n"),
		},

		// R3.3: --header default (1 line).
		{
			Name:  "header_default",
			Args:  []string{"--to=iec", "--header"},
			Stdin: []byte("size\n1048576\n"),
		},
		// R3.3: --header=2 (2 header lines).
		{
			Name:  "header_2",
			Args:  []string{"--to=si", "--header=2"},
			Stdin: []byte("col1\ncol2\n1000\n"),
		},
		// R3.3: --header combined with --field.
		{
			Name:  "header_with_field",
			Args:  []string{"--to=iec", "--header", "--field=2"},
			Stdin: []byte("name size\nfoo 1048576\n"),
		},

		// R3.4: --from-unit scales input.
		{
			Name:  "from_unit_1024",
			Args:  []string{"--from-unit=1024", "--to=iec"},
			Stdin: []byte("1\n"),
		},
		// R3.4: --to-unit scales output.
		{
			Name:  "to_unit_1000",
			Args:  []string{"--to-unit=1000"},
			Stdin: []byte("5000\n"),
		},
		// R3.4: --from-unit combined with --to.
		{
			Name:  "from_unit_with_to_si",
			Args:  []string{"--from-unit=1024", "--to=si"},
			Stdin: []byte("5\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
