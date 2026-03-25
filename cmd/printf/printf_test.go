// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd073-printf R4.1–R4.4.
// Compares Go printf binary against gprintf (GNU printf from Homebrew coreutils).
package main

import (
	"bytes"
	"os/exec"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gprintf")
	if err != nil {
		t.Skip("reference binary gprintf not in PATH")
	}

	tests := []testutils.DiffTest{}
	tests = append(tests, integerFormatTests()...)
	tests = append(tests, floatFormatTests()...)
	tests = append(tests, stringCharTests()...)
	tests = append(tests, escapeSequenceTests()...)
	tests = append(tests, percentBTests()...)
	tests = append(tests, widthPrecisionTests()...)
	tests = append(tests, flagTests()...)
	tests = append(tests, starWidthPrecTests()...)
	tests = append(tests, argCyclingTests()...)
	tests = append(tests, missingArgTests()...)
	tests = append(tests, numericArgFormatTests()...)
	tests = append(tests, errorTests()...)

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// progNameRe matches the program name prefix in error messages.
var progNameRe = regexp.MustCompile(`^(gprintf|printf): `)

// normalizeErrMsg replaces gprintf:/printf: prefixes so error messages
// from both binaries can be compared despite program name differences.
func normalizeErrMsg(data []byte) []byte {
	return progNameRe.ReplaceAll(data, []byte("printf: "))
}

// normalizeFirstLine keeps only the first line of output to ignore
// GNU's "Try ... --help" suffix in error messages.
func normalizeFirstLine(data []byte) []byte {
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		return data[:idx+1]
	}
	return data
}

// integerFormatTests covers R4.1: %d, %i, %o, %u, %x, %X.
func integerFormatTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "d_decimal",
			Args: []string{"%d\n", "42"},
		},
		{
			Name: "i_decimal",
			Args: []string{"%i\n", "42"},
		},
		{
			Name: "o_octal",
			Args: []string{"%o\n", "255"},
		},
		{
			Name: "u_unsigned",
			Args: []string{"%u\n", "42"},
		},
		{
			Name: "x_lowercase_hex",
			Args: []string{"%x\n", "255"},
		},
		{
			Name: "X_uppercase_hex",
			Args: []string{"%X\n", "255"},
		},
		{
			Name: "d_negative",
			Args: []string{"%d\n", "-7"},
		},
		{
			Name: "d_zero",
			Args: []string{"%d\n", "0"},
		},
		{
			Name: "hex_and_octal_combined",
			Args: []string{"%x %o\n", "255", "255"},
		},
	}
}

// floatFormatTests covers R4.1: %f, %e, %g and uppercase variants.
func floatFormatTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "f_float",
			Args: []string{"%f\n", "3.14159"},
		},
		{
			Name: "e_scientific",
			Args: []string{"%e\n", "3.14159"},
		},
		{
			Name: "g_general",
			Args: []string{"%g\n", "3.14159"},
		},
		{
			Name: "F_uppercase_float",
			Args: []string{"%F\n", "3.14159"},
		},
		{
			Name: "E_uppercase_scientific",
			Args: []string{"%E\n", "3.14159"},
		},
		{
			Name: "G_uppercase_general",
			Args: []string{"%G\n", "3.14159"},
		},
		{
			Name: "f_zero",
			Args: []string{"%f\n", "0"},
		},
		{
			Name: "f_negative",
			Args: []string{"%f\n", "-2.5"},
		},
		{
			Name: "g_large_number",
			Args: []string{"%g\n", "100000.0"},
		},
		{
			Name: "e_small_number",
			Args: []string{"%e\n", "0.00001"},
		},
	}
}

// stringCharTests covers R4.1: %s, %c.
func stringCharTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "s_string",
			Args: []string{"%s\n", "hello"},
		},
		{
			Name: "c_char",
			Args: []string{"%c\n", "A"},
		},
		{
			Name: "s_empty_string",
			Args: []string{"%s\n", ""},
		},
		{
			Name: "c_from_word",
			Args: []string{"%c\n", "hello"},
		},
		{
			Name: "d_and_s_combined",
			Args: []string{"%d %s\n", "42", "hello"},
		},
	}
}

// escapeSequenceTests covers R4.2: escape sequences in FORMAT.
func escapeSequenceTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "escape_newline",
			Args: []string{"a\\nb"},
		},
		{
			Name: "escape_tab",
			Args: []string{"a\\tb"},
		},
		{
			Name: "escape_backslash",
			Args: []string{"a\\\\b"},
		},
		{
			Name: "escape_tab_newline",
			Args: []string{"a\\tb\\n"},
		},
		{
			Name: "escape_octal",
			Args: []string{"\\101\\102\\103\\n"},
		},
		{
			Name: "escape_hex",
			Args: []string{"\\x41\\x42\\x43\\n"},
		},
		{
			Name: "escape_alert_backspace",
			Args: []string{"\\a\\b"},
		},
		{
			Name: "escape_cr_ff_vt",
			Args: []string{"\\r\\f\\v"},
		},
		{
			Name: "literal_percent",
			Args: []string{"100%%\\n"},
		},
	}
}

// percentBTests covers R4.2: %b argument escapes including \c.
func percentBTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "b_newline_escape",
			Args: []string{"%b", "hello\\nworld"},
		},
		{
			Name: "b_tab_escape",
			Args: []string{"%b", "col1\\tcol2"},
		},
		{
			Name: "b_backslash_escape",
			Args: []string{"%b", "a\\\\b"},
		},
		{
			Name: "b_octal_escape",
			Args: []string{"%b", "\\0101"},
		},
		{
			Name: "b_hex_escape",
			Args: []string{"%b", "\\x41"},
		},
		{
			Name: "b_stop_output",
			Args: []string{"%b", "hello\\cworld"},
		},
		{
			Name: "b_stop_with_remaining_args",
			Args: []string{"%b %s\n", "stop\\c", "ignored"},
		},
	}
}

// widthPrecisionTests covers R4.3: width and precision combinations.
func widthPrecisionTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "width_d",
			Args: []string{"%10d\n", "42"},
		},
		{
			Name: "width_s",
			Args: []string{"%10s\n", "hi"},
		},
		{
			Name: "precision_f",
			Args: []string{"%.5f\n", "3.14159"},
		},
		{
			Name: "precision_s",
			Args: []string{"%.3s\n", "hello"},
		},
		{
			Name: "width_precision_f",
			Args: []string{"%10.2f\n", "3.14159"},
		},
		{
			Name: "width_precision_s",
			Args: []string{"%10.3s\n", "hello"},
		},
		{
			Name: "zero_pad_float",
			Args: []string{"%010.2f\n", "3.14159"},
		},
		{
			Name: "precision_zero_f",
			Args: []string{"%.0f\n", "3.7"},
		},
	}
}

// flagTests covers R4.3: flag combinations.
func flagTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "flag_left_align_s",
			Args: []string{"%-10s|\n", "hi"},
		},
		{
			Name: "flag_left_align_d",
			Args: []string{"%-10d|\n", "42"},
		},
		{
			Name: "flag_plus_d",
			Args: []string{"%+d\n", "42"},
		},
		{
			Name: "flag_plus_negative_d",
			Args: []string{"%+d\n", "-42"},
		},
		{
			Name: "flag_space_d",
			Args: []string{"% d\n", "42"},
		},
		{
			Name: "flag_zero_pad_d",
			Args: []string{"%08d\n", "42"},
		},
		{
			Name: "flag_zero_pad_x",
			Args: []string{"%08x\n", "255"},
		},
		{
			Name: "flag_hash_o",
			Args: []string{"%#o\n", "42"},
		},
		{
			Name: "flag_hash_x",
			Args: []string{"%#x\n", "42"},
		},
		{
			Name: "flag_hash_X",
			Args: []string{"%#X\n", "42"},
		},
		{
			Name: "flag_combined_plus_zero",
			Args: []string{"%+08d\n", "42"},
		},
	}
}

// starWidthPrecTests covers R4.3: '*' for width and precision.
func starWidthPrecTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "star_width_d",
			Args: []string{"%*d\n", "10", "42"},
		},
		{
			Name: "star_precision_f",
			Args: []string{"%.*f\n", "3", "3.14159"},
		},
		{
			Name: "star_width_and_precision_f",
			Args: []string{"%*.*f\n", "10", "2", "3.14159"},
		},
		{
			Name: "star_width_s",
			Args: []string{"%*s\n", "10", "hi"},
		},
		{
			Name: "star_negative_width",
			Args: []string{"%*d|\n", "-10", "42"},
		},
	}
}

// argCyclingTests covers R4.4: argument recycling.
func argCyclingTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "cycle_s",
			Args: []string{"%s\n", "a", "b", "c"},
		},
		{
			Name: "cycle_d",
			Args: []string{"%d\n", "1", "2", "3", "4"},
		},
		{
			Name: "cycle_two_specifiers",
			Args: []string{"%s=%d\n", "a", "1", "b", "2", "c", "3"},
		},
		{
			Name: "no_args_no_specifiers",
			Args: []string{"hello world\\n"},
		},
	}
}

// missingArgTests covers R4.4: missing arguments default to 0/"".
func missingArgTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "missing_d_arg",
			Args: []string{"%d\n"},
		},
		{
			Name: "missing_s_arg",
			Args: []string{"%s\n"},
		},
		{
			Name: "missing_f_arg",
			Args: []string{"%f\n"},
		},
		{
			Name: "missing_second_arg",
			Args: []string{"%s %d\n", "hello"},
		},
		{
			Name: "missing_c_arg",
			Args: []string{"%c"},
		},
	}
}

// numericArgFormatTests covers R4.4: hex, octal, and character arg formats.
func numericArgFormatTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{
			Name: "hex_arg_0x",
			Args: []string{"%d\n", "0xff"},
		},
		{
			Name: "octal_arg_0",
			Args: []string{"%d\n", "077"},
		},
		{
			Name: "char_value_double_quote",
			Args: []string{"%d\n", "\"A"},
		},
		{
			Name: "char_value_single_quote",
			Args: []string{"%d\n", "'A"},
		},
		{
			Name: "char_value_space",
			Args: []string{"%d\n", "' "},
		},
		{
			Name: "hex_arg_uppercase",
			Args: []string{"%d\n", "0xFF"},
		},
	}
}

// errorTests covers R4.4: error cases with exit code 1.
func errorTests() []testutils.DiffTest {
	norm := []testutils.NormalizeFunc{normalizeErrMsg, normalizeFirstLine}
	return []testutils.DiffTest{
		{
			Name:      "error_non_numeric_d",
			Args:      []string{"%d\n", "abc"},
			Normalize: norm,
		},
		{
			Name:      "error_non_numeric_f",
			Args:      []string{"%f\n", "notanumber"},
			Normalize: norm,
		},
		{
			Name:      "error_missing_format",
			Args:      []string{},
			Normalize: norm,
		},
	}
}
