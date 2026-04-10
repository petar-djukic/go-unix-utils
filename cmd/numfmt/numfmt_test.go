// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/numfmt against gnumfmt reference binary.
// Implements srd071-numfmt R4.3, R4.4.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normProgName replaces gnumfmt program name with numfmt for stderr comparison.
func normProgName(b []byte) []byte {
	return bytes.Replace(b, []byte("gnumfmt:"), []byte("numfmt:"), -1)
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gnumfmt")
	if err != nil {
		t.Skipf("reference binary gnumfmt not in PATH: %v", err)
	}
	tests := buildTestCases()
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func buildTestCases() []testutils.DiffTest {
	norm := []testutils.NormalizeFunc{normProgName}
	tests := []testutils.DiffTest{}
	tests = append(tests, toConversionTests()...)
	tests = append(tests, fromConversionTests()...)
	tests = append(tests, formatTests()...)
	tests = append(tests, fieldAndHeaderTests()...)
	tests = append(tests, invalidModeTests(norm)...)
	tests = append(tests, zeroTerminatedTests()...)
	tests = append(tests, errorTests(norm)...)
	tests = append(tests, operandTests()...)
	return tests
}

func toConversionTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "to_si_1000", Args: []string{"--to=si"}, Stdin: []byte("1000\n")},
		{Name: "to_si_1500", Args: []string{"--to=si"}, Stdin: []byte("1500\n")},
		{Name: "to_si_1000000", Args: []string{"--to=si"}, Stdin: []byte("1000000\n")},
		{Name: "to_iec_1024", Args: []string{"--to=iec"}, Stdin: []byte("1024\n")},
		{Name: "to_iec_1048576", Args: []string{"--to=iec"}, Stdin: []byte("1048576\n")},
		{Name: "to_iec_i_1024", Args: []string{"--to=iec-i"}, Stdin: []byte("1024\n")},
		{Name: "to_iec_i_1048576", Args: []string{"--to=iec-i"}, Stdin: []byte("1048576\n")},
		{Name: "to_none", Args: []string{"--to=none"}, Stdin: []byte("1000\n")},
		{Name: "passthrough", Stdin: []byte("42\n")},
		{Name: "multi_line", Args: []string{"--to=si"}, Stdin: []byte("1000\n2000\n3000\n")},
	}
}

func fromConversionTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "from_si_1K", Args: []string{"--from=si"}, Stdin: []byte("1K\n")},
		{Name: "from_si_1M", Args: []string{"--from=si"}, Stdin: []byte("1M\n")},
		{Name: "from_iec_1K", Args: []string{"--from=iec"}, Stdin: []byte("1K\n")},
		{Name: "from_iec_1M", Args: []string{"--from=iec"}, Stdin: []byte("1M\n")},
		{Name: "from_iec_i_1Ki", Args: []string{"--from=iec-i"}, Stdin: []byte("1Ki\n")},
		{Name: "from_auto_1K", Args: []string{"--from=auto"}, Stdin: []byte("1K\n")},
		{Name: "from_auto_1Ki", Args: []string{"--from=auto"}, Stdin: []byte("1Ki\n")},
		{Name: "from_si_to_iec", Args: []string{"--from=si", "--to=iec"}, Stdin: []byte("1000000\n")},
		{Name: "from_si_to_si", Args: []string{"--from=si", "--to=si"}, Stdin: []byte("1K\n")},
	}
}

func formatTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "format_width", Args: []string{"--to=si", "--format=%10f"}, Stdin: []byte("1000\n")},
		{Name: "format_precision", Args: []string{"--to=si", "--format=%.2f"}, Stdin: []byte("1000\n")},
		{Name: "padding_right", Args: []string{"--to=si", "--padding=10"}, Stdin: []byte("1000\n")},
		{Name: "padding_left", Args: []string{"--to=si", "--padding=-10"}, Stdin: []byte("1000\n")},
		{Name: "round_up", Args: []string{"--to=si", "--round=up"}, Stdin: []byte("1500\n")},
		{Name: "round_down", Args: []string{"--to=si", "--round=down"}, Stdin: []byte("1500\n")},
		{Name: "round_from_zero", Args: []string{"--to=si", "--round=from-zero"}, Stdin: []byte("1500\n")},
		{Name: "round_towards_zero", Args: []string{"--to=si", "--round=towards-zero"}, Stdin: []byte("1500\n")},
		{Name: "round_nearest", Args: []string{"--to=si", "--round=nearest"}, Stdin: []byte("1500\n")},
		{Name: "suffix_B", Args: []string{"--to=si", "--suffix=B"}, Stdin: []byte("1000\n")},
		{Name: "from_unit", Args: []string{"--from-unit=1024", "--to=iec"}, Stdin: []byte("1\n")},
		{Name: "to_unit", Args: []string{"--to-unit=1000", "--to=si"}, Stdin: []byte("1000000\n")},
	}
}

func fieldAndHeaderTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "field_2", Args: []string{"--to=si", "--field=2"}, Stdin: []byte("foo 1000\n")},
		{Name: "delimiter_comma", Args: []string{"--to=si", "--delimiter=,", "--field=2"}, Stdin: []byte("foo,1000\n")},
		{Name: "delimiter_short", Args: []string{"--to=si", "-d,", "--field=2"}, Stdin: []byte("foo,1000\n")},
		{Name: "header_default", Args: []string{"--to=si", "--header"}, Stdin: []byte("name value\n1000\n2000\n")},
		{Name: "header_2", Args: []string{"--to=si", "--header=2"}, Stdin: []byte("h1\nh2\n1000\n")},
	}
}

func invalidModeTests(norm []testutils.NormalizeFunc) []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:      "invalid_abort_single",
			Args:      []string{"--to=si", "--invalid=abort"},
			Stdin:     []byte("abc\n"),
			ExitCode:  2,
			Normalize: norm,
		},
		{
			Name:      "invalid_abort_multi",
			Args:      []string{"--to=si", "--invalid=abort"},
			Stdin:     []byte("1000\nabc\n2000\n"),
			ExitCode:  2,
			Normalize: norm,
		},
		{
			Name:      "invalid_fail_multi",
			Args:      []string{"--to=si", "--invalid=fail"},
			Stdin:     []byte("1000\nabc\n2000\n"),
			ExitCode:  2,
			Normalize: norm,
		},
		{
			Name:      "invalid_warn",
			Args:      []string{"--to=si", "--invalid=warn"},
			Stdin:     []byte("abc\n"),
			Normalize: norm,
		},
		{
			Name:      "invalid_warn_multi",
			Args:      []string{"--to=si", "--invalid=warn"},
			Stdin:     []byte("1000\nabc\n2000\n"),
			Normalize: norm,
		},
		{
			Name:  "invalid_ignore",
			Args:  []string{"--to=si", "--invalid=ignore"},
			Stdin: []byte("abc\n"),
		},
		{
			Name:  "invalid_ignore_multi",
			Args:  []string{"--to=si", "--invalid=ignore"},
			Stdin: []byte("1000\nabc\n2000\n"),
		},
	}
}

func zeroTerminatedTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:  "zero_term_z",
			Args:  []string{"--to=si", "-z"},
			Stdin: []byte("1000\x002000\x00"),
		},
		{
			Name:  "zero_term_long",
			Args:  []string{"--to=si", "--zero-terminated"},
			Stdin: []byte("1000\x002000\x00"),
		},
	}
}

func errorTests(norm []testutils.NormalizeFunc) []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name:      "invalid_number_default",
			Args:      []string{"--to=si"},
			Stdin:     []byte("abc\n"),
			ExitCode:  2,
			Normalize: norm,
		},
		{
			Name:      "invalid_suffix",
			Args:      []string{"--from=si"},
			Stdin:     []byte("1X\n"),
			ExitCode:  2,
			Normalize: norm,
		},
	}
}

func operandTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "operand_single", Args: []string{"--to=si", "1000"}},
		{Name: "operand_multiple", Args: []string{"--to=si", "1000", "2000000"}},
		{Name: "operand_from", Args: []string{"--from=si", "1K"}},
	}
}
