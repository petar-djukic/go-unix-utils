// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/numfmt against gnumfmt (GNU coreutils).
// Covers prd071-numfmt R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
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
	tests := buildDiffTests()
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildDiffTests assembles all differential test cases for numfmt.
func buildDiffTests() []testutils.DiffTest {
	var tests []testutils.DiffTest
	tests = append(tests, toConversionTests()...)
	tests = append(tests, fromConversionTests()...)
	tests = append(tests, passthroughTests()...)
	tests = append(tests, operandTests()...)
	tests = append(tests, formatTests()...)
	tests = append(tests, paddingTests()...)
	tests = append(tests, roundTests()...)
	tests = append(tests, suffixTests()...)
	tests = append(tests, fieldTests()...)
	tests = append(tests, delimiterTests()...)
	tests = append(tests, headerTests()...)
	tests = append(tests, unitScaleTests()...)
	tests = append(tests, errorTests()...)
	tests = append(tests, combinedFlagTests()...)
	return tests
}

// toConversionTests covers R1.1: --to=si/iec/iec-i conversion.
func toConversionTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "to-si-1000", Args: []string{"--to=si"}, Stdin: []byte("1000\n")},
		{Name: "to-si-1500000", Args: []string{"--to=si"}, Stdin: []byte("1500000\n")},
		{Name: "to-si-999", Args: []string{"--to=si"}, Stdin: []byte("999\n")},
		{Name: "to-si-zero", Args: []string{"--to=si"}, Stdin: []byte("0\n")},
		{Name: "to-si-negative", Args: []string{"--to=si"}, Stdin: []byte("-1000\n")},
		{Name: "to-si-large", Args: []string{"--to=si"}, Stdin: []byte("1000000000\n")},
		{Name: "to-iec-1048576", Args: []string{"--to=iec"}, Stdin: []byte("1048576\n")},
		{Name: "to-iec-1024", Args: []string{"--to=iec"}, Stdin: []byte("1024\n")},
		{Name: "to-iec-500", Args: []string{"--to=iec"}, Stdin: []byte("500\n")},
		{Name: "to-iec-large", Args: []string{"--to=iec"}, Stdin: []byte("1073741824\n")},
		{Name: "to-iec-i-1048576", Args: []string{"--to=iec-i"}, Stdin: []byte("1048576\n")},
		{Name: "to-iec-i-1024", Args: []string{"--to=iec-i"}, Stdin: []byte("1024\n")},
	}
}

// fromConversionTests covers R1.2: --from=si/iec/iec-i conversion.
func fromConversionTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "from-si-1K", Args: []string{"--from=si"}, Stdin: []byte("1K\n")},
		{Name: "from-si-1.5M", Args: []string{"--from=si"}, Stdin: []byte("1.5M\n")},
		{Name: "from-si-plain", Args: []string{"--from=si"}, Stdin: []byte("42\n")},
		{Name: "from-si-1G", Args: []string{"--from=si"}, Stdin: []byte("1G\n")},
		{Name: "from-iec-1K", Args: []string{"--from=iec"}, Stdin: []byte("1K\n")},
		{Name: "from-iec-1M", Args: []string{"--from=iec"}, Stdin: []byte("1M\n")},
		{Name: "from-iec-i-1Ki", Args: []string{"--from=iec-i"}, Stdin: []byte("1Ki\n")},
		{Name: "from-si-to-iec", Args: []string{"--from=si", "--to=iec"}, Stdin: []byte("1K\n")},
	}
}

// passthroughTests covers R1.3: no conversion when no flags are given.
func passthroughTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "passthrough-integer", Stdin: []byte("42\n")},
		{Name: "passthrough-float", Stdin: []byte("3.14\n")},
	}
}

// operandTests covers R1.4: operand and stdin input modes.
func operandTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "operand-to-si", Args: []string{"--to=si", "1000"}},
		{Name: "operand-multiple", Args: []string{"--to=si", "1000", "2000000"}},
		{Name: "multi-line", Args: []string{"--to=si"}, Stdin: []byte("1000\n2000\n3000\n")},
	}
}

// formatTests covers R2.1: --format with width and precision.
func formatTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "format-precision", Args: []string{"--to=si", "--format=%.2f"}, Stdin: []byte("1500000\n")},
		{Name: "format-width", Args: []string{"--to=si", "--format=%10f"}, Stdin: []byte("1500000\n")},
		{Name: "format-width-prec", Args: []string{"--to=si", "--format=%10.2f"}, Stdin: []byte("1500000\n")},
		{Name: "format-left-align", Args: []string{"--to=si", "--format=%-10f"}, Stdin: []byte("1500000\n")},
	}
}

// paddingTests covers R2.2: --padding for output alignment.
func paddingTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "padding-right", Args: []string{"--padding=10", "--to=si"}, Stdin: []byte("1000\n")},
		{Name: "padding-left", Args: []string{"--padding=-10", "--to=si"}, Stdin: []byte("1000\n")},
		{Name: "padding-no-effect", Args: []string{"--padding=3", "--to=si"}, Stdin: []byte("1000000000\n")},
	}
}

// roundTests covers R2.3: --round modes.
func roundTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "round-up", Args: []string{"--to=si", "--round=up"}, Stdin: []byte("1001\n")},
		{Name: "round-down", Args: []string{"--to=si", "--round=down"}, Stdin: []byte("1999\n")},
		{Name: "round-from-zero", Args: []string{"--to=si", "--round=from-zero"}, Stdin: []byte("1500\n")},
		{Name: "round-towards-zero", Args: []string{"--to=si", "--round=towards-zero"}, Stdin: []byte("1999\n")},
		{Name: "round-nearest", Args: []string{"--to=si", "--round=nearest"}, Stdin: []byte("1500\n")},
		{Name: "round-up-large", Args: []string{"--to=iec", "--round=up"}, Stdin: []byte("1048577\n")},
		{Name: "round-down-large", Args: []string{"--to=iec", "--round=down"}, Stdin: []byte("1572863\n")},
		{Name: "round-neg-from-zero", Args: []string{"--to=si", "--round=from-zero"}, Stdin: []byte("-1500\n")},
		{Name: "round-neg-towards-zero", Args: []string{"--to=si", "--round=towards-zero"}, Stdin: []byte("-1999\n")},
	}
}

// suffixTests covers R2.4: --suffix appended to output.
func suffixTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "suffix-output", Args: []string{"--to=iec", "--suffix=B"}, Stdin: []byte("1048576\n")},
		{Name: "suffix-input-output", Args: []string{"--from=si", "--to=si", "--suffix=B"}, Stdin: []byte("1kB\n")},
		{Name: "suffix-passthrough", Args: []string{"--to=si", "--suffix=B"}, Stdin: []byte("1000\n")},
	}
}

// fieldTests covers R3.1: --field selection.
func fieldTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "field-2", Args: []string{"--to=si", "--field=2"}, Stdin: []byte("name 1000\n")},
		{Name: "field-range", Args: []string{"--to=si", "--field=2-3"}, Stdin: []byte("foo 1000 2000 bar\n")},
		{Name: "field-1", Args: []string{"--to=si", "--field=1"}, Stdin: []byte("1000 text\n")},
		{Name: "field-open-end", Args: []string{"--to=si", "--field=2-"}, Stdin: []byte("foo 1000 2000\n")},
	}
}

// delimiterTests covers R3.2: -d/--delimiter for field splitting.
func delimiterTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "delimiter-comma", Args: []string{"--to=si", "--field=2", "--delimiter=,"}, Stdin: []byte("name,1000\n")},
		{Name: "delimiter-short-d", Args: []string{"--to=si", "--field=2", "-d,"}, Stdin: []byte("name,1000\n")},
		{Name: "delimiter-tab", Args: []string{"--to=si", "--field=2", "-d\t"}, Stdin: []byte("name\t1000\n")},
		{Name: "delimiter-colon", Args: []string{"--to=si", "--field=2", "-d:"}, Stdin: []byte("name:1000\n")},
	}
}

// headerTests covers R3.3: --header pass-through.
func headerTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "header-default", Args: []string{"--to=si", "--header", "--field=2"}, Stdin: []byte("name value\nfoo 1000\nbar 2000\n")},
		{Name: "header-2", Args: []string{"--to=si", "--header=2", "--field=2"}, Stdin: []byte("h1 h2\nh3 h4\nfoo 1000\n")},
	}
}

// unitScaleTests covers R3.4: --from-unit and --to-unit scaling.
func unitScaleTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "from-unit", Args: []string{"--from-unit=1024", "--to=iec"}, Stdin: []byte("5\n")},
		{Name: "to-unit", Args: []string{"--to=si", "--to-unit=1000"}, Stdin: []byte("5000000\n")},
		{Name: "from-unit-to-si", Args: []string{"--from-unit=1000000", "--to=si"}, Stdin: []byte("5\n")},
	}
}

// errorTests covers R4.2: exit code 2 on invalid input and error cases.
func errorTests() []testutils.DiffTest {
	norm := []testutils.NormalizeFunc{normProgName}
	return []testutils.DiffTest{
		// R4.2: invalid number with no conversion flags
		{Name: "invalid-number", Stdin: []byte("abc\n"), ExitCode: 2, Normalize: norm},
		// R4.2: invalid number with --to
		{Name: "invalid-number-to-si", Args: []string{"--to=si"}, Stdin: []byte("abc\n"),
			ExitCode: 2, Normalize: norm},
		// R4.2: invalid number as operand
		{Name: "invalid-operand", Args: []string{"--to=si", "notanumber"},
			ExitCode: 2, Normalize: norm},
		// R4.2: empty input line treated as invalid
		{Name: "empty-input-line", Args: []string{"--to=si"}, Stdin: []byte("\n"),
			ExitCode: 2, Normalize: norm},
	}
}

// combinedFlagTests covers R4.4: comprehensive flag combinations per R4.4.
func combinedFlagTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		// R4.1, R4.4: --to + --padding + --round combined, exit 0
		{Name: "combined-to-padding-round",
			Args:  []string{"--to=si", "--padding=10", "--round=up"},
			Stdin: []byte("1234\n")},
		// R4.1, R4.4: --from + --to + --suffix combined, exit 0
		{Name: "combined-from-to-suffix",
			Args:  []string{"--from=si", "--to=iec", "--suffix=B"},
			Stdin: []byte("1kB\n")},
		// R4.4: --to + --format + --suffix combined
		{Name: "combined-to-format-suffix",
			Args:  []string{"--to=si", "--format=%.2f", "--suffix=B"},
			Stdin: []byte("1500000\n")},
		// R4.4: --to + --field + --header + --delimiter combined
		{Name: "combined-field-header-delim",
			Args:  []string{"--to=iec", "--field=2", "--header", "-d,"},
			Stdin: []byte("name,size\nfoo,1048576\n")},
		// R4.4: --from-unit + --to-unit + --to combined
		{Name: "combined-from-unit-to-unit",
			Args:  []string{"--from-unit=1024", "--to-unit=1", "--to=iec"},
			Stdin: []byte("1024\n")},
		// R4.4: --to=iec-i + --round + --format combined
		{Name: "combined-iec-i-round-format",
			Args:  []string{"--to=iec-i", "--round=down", "--format=%.1f"},
			Stdin: []byte("1572864\n")},
		// R4.4: multiple operands with --to and --suffix
		{Name: "combined-operands-to-suffix",
			Args: []string{"--to=si", "--suffix=B", "1000", "2000000"}},
		// R4.4: --to=none passthrough with --padding
		{Name: "combined-to-none-padding",
			Args:  []string{"--to=none", "--padding=10"},
			Stdin: []byte("42\n")},
	}
}
