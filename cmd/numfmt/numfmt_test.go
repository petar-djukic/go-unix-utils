// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/numfmt against gnumfmt (GNU coreutils).
// Covers prd071-numfmt R1.1-R1.4 (core conversion), R2.1 (--format), R2.2 (--padding).
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normProgName normalizes the program name in error output so "gnumfmt:"
// and "numfmt:" compare equal in differential tests.
func normProgName(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("gnumfmt:"), []byte("numfmt:"))
}

// TestDiff runs differential tests comparing the Go numfmt binary against
// the GNU reference binary (gnumfmt).
// D4: uses testutils.BuildBinary and exec.LookPath per shared protocol.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnumfmt")
	if err != nil {
		t.Skip("reference binary gnumfmt not in PATH")
	}
	tests := []testutils.DiffTest{
		// R1.1: --to=si conversion
		{Name: "to-si-1000", Args: []string{"--to=si"}, Stdin: []byte("1000\n")},
		{Name: "to-si-1500000", Args: []string{"--to=si"}, Stdin: []byte("1500000\n")},
		{Name: "to-si-999", Args: []string{"--to=si"}, Stdin: []byte("999\n")},
		{Name: "to-si-zero", Args: []string{"--to=si"}, Stdin: []byte("0\n")},
		{Name: "to-si-negative", Args: []string{"--to=si"}, Stdin: []byte("-1000\n")},
		// R1.1: --to=iec conversion
		{Name: "to-iec-1048576", Args: []string{"--to=iec"}, Stdin: []byte("1048576\n")},
		{Name: "to-iec-1024", Args: []string{"--to=iec"}, Stdin: []byte("1024\n")},
		{Name: "to-iec-500", Args: []string{"--to=iec"}, Stdin: []byte("500\n")},
		// R1.1: --to=iec-i conversion
		{Name: "to-iec-i-1048576", Args: []string{"--to=iec-i"}, Stdin: []byte("1048576\n")},
		// R1.2: --from=si conversion
		{Name: "from-si-1K", Args: []string{"--from=si"}, Stdin: []byte("1K\n")},
		{Name: "from-si-1.5M", Args: []string{"--from=si"}, Stdin: []byte("1.5M\n")},
		{Name: "from-si-plain", Args: []string{"--from=si"}, Stdin: []byte("42\n")},
		// R1.2: --from=iec conversion
		{Name: "from-iec-1K", Args: []string{"--from=iec"}, Stdin: []byte("1K\n")},
		// R1.2: --from + --to combined
		{Name: "from-si-to-iec", Args: []string{"--from=si", "--to=iec"}, Stdin: []byte("1K\n")},
		// R1.3: passthrough (no --from or --to)
		{Name: "passthrough-integer", Stdin: []byte("42\n")},
		{Name: "passthrough-float", Stdin: []byte("3.14\n")},
		// R1.4: operand input
		{Name: "operand-to-si", Args: []string{"--to=si", "1000"}},
		{Name: "operand-multiple", Args: []string{"--to=si", "1000", "2000000"}},
		// R1.4: multiple stdin lines
		{Name: "multi-line", Args: []string{"--to=si"}, Stdin: []byte("1000\n2000\n3000\n")},
		// R2.1: --format with precision
		{Name: "format-precision", Args: []string{"--to=si", "--format=%.2f"}, Stdin: []byte("1500000\n")},
		// R2.1: --format with width
		{Name: "format-width", Args: []string{"--to=si", "--format=%10f"}, Stdin: []byte("1500000\n")},
		// R2.1: --format with width and precision
		{Name: "format-width-prec", Args: []string{"--to=si", "--format=%10.2f"}, Stdin: []byte("1500000\n")},
		// R2.2: --padding positive (right-align)
		{Name: "padding-right", Args: []string{"--padding=10", "--to=si"}, Stdin: []byte("1000\n")},
		// R2.2: --padding negative (left-align)
		{Name: "padding-left", Args: []string{"--padding=-10", "--to=si"}, Stdin: []byte("1000\n")},
		// Error: invalid number (normalizer handles gnumfmt vs numfmt name)
		{Name: "invalid-number", Stdin: []byte("abc\n"), ExitCode: 2,
			Normalize: []testutils.NormalizeFunc{normProgName}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
