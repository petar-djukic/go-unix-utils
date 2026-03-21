// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/numfmt against gnumfmt reference binary.
// Tests prd071-numfmt R1.1–R1.4.
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
