// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/numfmt.
// Traces: prd071-numfmt R1.1, R1.2, R1.3, R1.4.
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
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
