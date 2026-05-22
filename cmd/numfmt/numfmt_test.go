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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
