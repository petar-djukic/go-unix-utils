// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/numfmt against gnumfmt (GNU coreutils).
// Covers prd071-numfmt R1.1-R1.4, R2.1-R2.4, R3.1-R3.4.
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
		// R2.3: --round modes
		{Name: "round-up", Args: []string{"--to=si", "--round=up"}, Stdin: []byte("1001\n")},
		{Name: "round-down", Args: []string{"--to=si", "--round=down"}, Stdin: []byte("1999\n")},
		{Name: "round-from-zero", Args: []string{"--to=si", "--round=from-zero"}, Stdin: []byte("1500\n")},
		{Name: "round-towards-zero", Args: []string{"--to=si", "--round=towards-zero"}, Stdin: []byte("1999\n")},
		{Name: "round-nearest", Args: []string{"--to=si", "--round=nearest"}, Stdin: []byte("1500\n")},
		{Name: "round-up-large", Args: []string{"--to=iec", "--round=up"}, Stdin: []byte("1048577\n")},
		{Name: "round-down-large", Args: []string{"--to=iec", "--round=down"}, Stdin: []byte("1572863\n")},
		// R2.4: --suffix
		{Name: "suffix-output", Args: []string{"--to=iec", "--suffix=B"}, Stdin: []byte("1048576\n")},
		{Name: "suffix-input-output", Args: []string{"--from=si", "--to=si", "--suffix=B"}, Stdin: []byte("1kB\n")},
		{Name: "suffix-passthrough", Args: []string{"--to=si", "--suffix=B"}, Stdin: []byte("1000\n")},
		// R3.1: --field selection
		{Name: "field-2", Args: []string{"--to=si", "--field=2"}, Stdin: []byte("name 1000\n")},
		{Name: "field-range", Args: []string{"--to=si", "--field=2-3"}, Stdin: []byte("foo 1000 2000 bar\n")},
		// R3.2: --delimiter
		{Name: "delimiter-comma", Args: []string{"--to=si", "--field=2", "--delimiter=,"}, Stdin: []byte("name,1000\n")},
		{Name: "delimiter-short-d", Args: []string{"--to=si", "--field=2", "-d,"}, Stdin: []byte("name,1000\n")},
		{Name: "delimiter-tab", Args: []string{"--to=si", "--field=2", "-d\t"}, Stdin: []byte("name\t1000\n")},
		// R3.3: --header
		{Name: "header-default", Args: []string{"--to=si", "--header", "--field=2"}, Stdin: []byte("name value\nfoo 1000\nbar 2000\n")},
		{Name: "header-2", Args: []string{"--to=si", "--header=2", "--field=2"}, Stdin: []byte("h1 h2\nh3 h4\nfoo 1000\n")},
		// R3.4: --from-unit and --to-unit
		{Name: "from-unit", Args: []string{"--from-unit=1024", "--to=iec"}, Stdin: []byte("5\n")},
		{Name: "to-unit", Args: []string{"--to=si", "--to-unit=1000"}, Stdin: []byte("5000000\n")},
		{Name: "from-unit-to-si", Args: []string{"--from-unit=1000000", "--to=si"}, Stdin: []byte("5\n")},
		// Error: invalid number
		{Name: "invalid-number", Stdin: []byte("abc\n"), ExitCode: 2,
			Normalize: []testutils.NormalizeFunc{normProgName}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
