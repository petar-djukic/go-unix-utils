// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var stderrNorm testutils.NormalizeFunc = func(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gnumfmt:"), []byte("numfmt:"))
	return b
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnumfmt")
	if err != nil {
		t.Skip("reference binary not found")
	}
	tests := []testutils.DiffTest{
		{
			Name:  "to_si_basic",
			Args:  []string{"--to=si"},
			Stdin: []byte("1000\n"),
		},
		{
			Name:  "to_si_mega",
			Args:  []string{"--to=si"},
			Stdin: []byte("1500000\n"),
		},
		{
			Name:  "to_si_giga",
			Args:  []string{"--to=si"},
			Stdin: []byte("1000000000\n"),
		},
		{
			Name:  "to_si_negative",
			Args:  []string{"--to=si"},
			Stdin: []byte("-1048576\n"),
		},
		{
			Name:  "to_si_small",
			Args:  []string{"--to=si"},
			Stdin: []byte("42\n"),
		},
		{
			Name:  "to_si_boundary_10k",
			Args:  []string{"--to=si"},
			Stdin: []byte("9999\n"),
		},
		{
			Name:  "to_iec_basic",
			Args:  []string{"--to=iec"},
			Stdin: []byte("1048576\n"),
		},
		{
			Name:  "to_iec_kilo",
			Args:  []string{"--to=iec"},
			Stdin: []byte("1536\n"),
		},
		{
			Name:  "to_iec_giga",
			Args:  []string{"--to=iec"},
			Stdin: []byte("1073741824\n"),
		},
		{
			Name:  "to_iec_i_basic",
			Args:  []string{"--to=iec-i"},
			Stdin: []byte("1048576\n"),
		},
		{
			Name:  "to_iec_i_kilo",
			Args:  []string{"--to=iec-i"},
			Stdin: []byte("1024\n"),
		},
		{
			Name:  "to_none",
			Args:  []string{"--to=none"},
			Stdin: []byte("42\n"),
		},
		{
			Name:  "from_si_K",
			Args:  []string{"--from=si"},
			Stdin: []byte("1K\n"),
		},
		{
			Name:  "from_si_k_lower",
			Args:  []string{"--from=si"},
			Stdin: []byte("1k\n"),
		},
		{
			Name:  "from_si_M",
			Args:  []string{"--from=si"},
			Stdin: []byte("1M\n"),
		},
		{
			Name:  "from_si_fractional",
			Args:  []string{"--from=si"},
			Stdin: []byte("1.5K\n"),
		},
		{
			Name:  "from_si_negative",
			Args:  []string{"--from=si"},
			Stdin: []byte("-1K\n"),
		},
		{
			Name:  "from_iec_K",
			Args:  []string{"--from=iec"},
			Stdin: []byte("1K\n"),
		},
		{
			Name:  "from_iec_M",
			Args:  []string{"--from=iec"},
			Stdin: []byte("1M\n"),
		},
		{
			Name:  "from_iec_i_Ki",
			Args:  []string{"--from=iec-i"},
			Stdin: []byte("1Ki\n"),
		},
		{
			Name:  "from_iec_i_Mi",
			Args:  []string{"--from=iec-i"},
			Stdin: []byte("1Mi\n"),
		},
		{
			Name:     "passthrough_no_flags",
			Stdin:    []byte("42\n"),
			ExitCode: 0,
		},
		{
			Name:  "passthrough_large",
			Stdin: []byte("1048576\n"),
		},
		{
			Name:  "stdin_multiline",
			Args:  []string{"--to=si"},
			Stdin: []byte("1000\n2000\n3000\n"),
		},
		{
			Name: "operand_single",
			Args: []string{"--to=si", "1000"},
		},
		{
			Name: "operand_multiple",
			Args: []string{"--to=si", "1000", "2000", "3000"},
		},
		{
			Name:  "format_width",
			Args:  []string{"--to=si", "--format=%10f"},
			Stdin: []byte("1500000\n"),
		},
		{
			Name:  "format_precision",
			Args:  []string{"--to=si", "--format=%.2f"},
			Stdin: []byte("1500000\n"),
		},
		{
			Name:  "format_width_prec",
			Args:  []string{"--to=si", "--format=%10.2f"},
			Stdin: []byte("1500000\n"),
		},
		{
			Name:  "format_left_align",
			Args:  []string{"--to=si", "--format=%-10f"},
			Stdin: []byte("1500000\n"),
		},
		{
			Name:  "padding_right",
			Args:  []string{"--to=si", "--padding=10"},
			Stdin: []byte("1500000\n"),
		},
		{
			Name:  "padding_left",
			Args:  []string{"--to=si", "--padding=-10"},
			Stdin: []byte("1500000\n"),
		},
		{
			Name:  "round_up",
			Args:  []string{"--to=si", "--round=up"},
			Stdin: []byte("1340000\n"),
		},
		{
			Name:  "round_down",
			Args:  []string{"--to=si", "--round=down"},
			Stdin: []byte("1340000\n"),
		},
		{
			Name:  "round_from_zero",
			Args:  []string{"--to=si", "--round=from-zero"},
			Stdin: []byte("1340000\n"),
		},
		{
			Name:  "round_towards_zero",
			Args:  []string{"--to=si", "--round=towards-zero"},
			Stdin: []byte("1340000\n"),
		},
		{
			Name:  "round_nearest",
			Args:  []string{"--to=si", "--round=nearest"},
			Stdin: []byte("1340000\n"),
		},
		{
			Name:  "suffix_basic",
			Args:  []string{"--to=si", "--suffix=B"},
			Stdin: []byte("1000\n"),
		},
		{
			Name:  "suffix_with_iec",
			Args:  []string{"--to=iec", "--suffix=B"},
			Stdin: []byte("1048576\n"),
		},
		{
			Name:      "error_invalid_number",
			Args:      []string{"--to=si"},
			Stdin:     []byte("abc\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{stderrNorm},
		},
		{
			Name:      "error_suffix_without_from",
			Args:      []string{"--from=none"},
			Stdin:     []byte("1K\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{stderrNorm},
		},
		{
			Name:      "error_invalid_suffix",
			Args:      []string{"--from=si"},
			Stdin:     []byte("1m\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{stderrNorm},
		},
		{
			Name:  "field_single",
			Args:  []string{"--to=si", "--field=2"},
			Stdin: []byte("name 1000\n"),
		},
		{
			Name:  "field_multiple",
			Args:  []string{"--to=si", "--field=2,3"},
			Stdin: []byte("name 1000 2000\n"),
		},
		{
			Name:  "field_range",
			Args:  []string{"--to=si", "--field=2-3"},
			Stdin: []byte("name 1000 2000\n"),
		},
		{
			Name:  "field_open_range",
			Args:  []string{"--to=si", "--field=2-"},
			Stdin: []byte("name 1000 2000 3000\n"),
		},
		{
			Name:  "field_first",
			Args:  []string{"--to=si", "--field=1"},
			Stdin: []byte("1000 name\n"),
		},
		{
			Name:  "field_passthrough",
			Args:  []string{"--to=si", "--field=2"},
			Stdin: []byte("keep 1048576 also\n"),
		},
		{
			Name:  "delimiter_colon",
			Args:  []string{"--to=si", "--field=2", "--delimiter=:"},
			Stdin: []byte("name:1000:foo\n"),
		},
		{
			Name:  "delimiter_comma",
			Args:  []string{"--to=si", "--field=2", "-d", ","},
			Stdin: []byte("name,1000,foo\n"),
		},
		{
			Name:  "delimiter_tab",
			Args:  []string{"--to=si", "--field=2", "--delimiter=\t"},
			Stdin: []byte("name\t1000\tfoo\n"),
		},
		{
			Name:  "header_default",
			Args:  []string{"--to=si", "--header"},
			Stdin: []byte("size\n1000\n2000\n"),
		},
		{
			Name:  "header_2",
			Args:  []string{"--to=si", "--header=2"},
			Stdin: []byte("title\nsize\n1000\n2000\n"),
		},
		{
			Name:  "header_field",
			Args:  []string{"--to=iec", "--header", "--field=2"},
			Stdin: []byte("name size\nfoo 1048576\n"),
		},
		{
			Name:  "to_unit_basic",
			Args:  []string{"--to=si", "--to-unit=1000"},
			Stdin: []byte("5000000\n"),
		},
		{
			Name:  "from_unit_basic",
			Args:  []string{"--to=si", "--from-unit=1024"},
			Stdin: []byte("1000\n"),
		},
		{
			Name:  "from_unit_to_unit",
			Args:  []string{"--to=si", "--from-unit=1024", "--to-unit=1000"},
			Stdin: []byte("1000\n"),
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
