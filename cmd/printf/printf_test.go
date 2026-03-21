// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd073-printf R4.3–R4.4: differential tests for format string
// parsing, conversion specifiers, width/precision/flags, escape sequences,
// and argument recycling.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNameNormalizer replaces the reference binary name with "printf"
// so stderr messages from gprintf and printf compare equal.
var stderrNameNormalizer testutils.NormalizeFunc = func(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gprintf:"), []byte("printf:"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gprintf")
	if err != nil {
		t.Skipf("reference binary gprintf not in PATH: %v", err)
	}
	tests := []testutils.DiffTest{
		// R1.1: literal text and format string basics
		{Name: "literal_only", Args: []string{"hello"}},
		{Name: "literal_with_newline", Args: []string{"hello\\n"}},
		{Name: "literal_with_tab", Args: []string{"hello\\tworld"}},
		{Name: "percent_literal", Args: []string{"100%%\\n"}},
		{Name: "empty_format", Args: []string{""}},

		// R1.2: integer conversion specifiers
		{Name: "int_d", Args: []string{"%d\\n", "42"}},
		{Name: "int_i", Args: []string{"%i\\n", "42"}},
		{Name: "int_o", Args: []string{"%o\\n", "42"}},
		{Name: "int_u", Args: []string{"%u\\n", "42"}},
		{Name: "int_x", Args: []string{"%x\\n", "255"}},
		{Name: "int_X", Args: []string{"%X\\n", "255"}},
		{Name: "int_negative_d", Args: []string{"%d\\n", "-42"}},
		{Name: "int_hex_input", Args: []string{"%d\\n", "0xff"}},
		{Name: "int_octal_input", Args: []string{"%d\\n", "077"}},
		{Name: "int_zero", Args: []string{"%d\\n", "0"}},

		// R1.3: floating-point conversion specifiers
		{Name: "float_f", Args: []string{"%f\\n", "3.14159"}},
		{Name: "float_e", Args: []string{"%e\\n", "3.14159"}},
		{Name: "float_g", Args: []string{"%g\\n", "3.14159"}},
		{Name: "float_E", Args: []string{"%E\\n", "3.14159"}},
		{Name: "float_G", Args: []string{"%G\\n", "3.14159"}},
		{Name: "float_zero", Args: []string{"%f\\n", "0"}},
		{Name: "float_negative", Args: []string{"%f\\n", "-1.5"}},

		// R1.4: string, char, %b specifiers
		{Name: "string_s", Args: []string{"%s\\n", "hello"}},
		{Name: "char_c", Args: []string{"%c\\n", "A"}},
		{Name: "char_c_string", Args: []string{"%c", "hello"}},
		{Name: "b_plain", Args: []string{"%b", "hello"}},
		{Name: "b_escape_n", Args: []string{"%b", "hello\\nworld"}},
		{Name: "b_escape_t", Args: []string{"%b", "hello\\tworld"}},
		{Name: "b_octal", Args: []string{"%b", "\\0101"}},
		{Name: "b_stop_c", Args: []string{"%b", "hello\\cworld"}},

		// R2.1: field width and precision
		{Name: "width_d", Args: []string{"%10d\\n", "42"}},
		{Name: "precision_f", Args: []string{"%.2f\\n", "3.14159"}},
		{Name: "width_precision_f", Args: []string{"%10.2f\\n", "3.14159"}},
		{Name: "precision_d", Args: []string{"%.5d\\n", "42"}},
		{Name: "width_precision_d", Args: []string{"%10.5d\\n", "42"}},
		{Name: "width_s", Args: []string{"%10s\\n", "hi"}},
		{Name: "width_precision_s", Args: []string{"%10.3s\\n", "hello"}},
		{Name: "precision_zero_f", Args: []string{"%.0f\\n", "3.14"}},
		{Name: "width_precision_e", Args: []string{"%15.3e\\n", "3.14159"}},
		{Name: "width_precision_g", Args: []string{"%10.4g\\n", "3.14159"}},
		{Name: "width_o", Args: []string{"%10o\\n", "42"}},
		{Name: "width_x", Args: []string{"%10x\\n", "255"}},
		{Name: "precision_x", Args: []string{"%.8x\\n", "255"}},
		{Name: "width_c", Args: []string{"%5c\\n", "A"}},

		// R2.2: flag characters
		{Name: "zero_pad_d", Args: []string{"%010d\\n", "42"}},
		{Name: "left_align_d", Args: []string{"%-10d|\\n", "42"}},
		{Name: "plus_sign_d", Args: []string{"%+d\\n", "42"}},
		{Name: "plus_sign_neg_d", Args: []string{"%+d\\n", "-42"}},
		{Name: "space_d", Args: []string{"% d\\n", "42"}},
		{Name: "space_neg_d", Args: []string{"% d\\n", "-42"}},
		{Name: "hash_o", Args: []string{"%#o\\n", "42"}},
		{Name: "hash_x", Args: []string{"%#x\\n", "42"}},
		{Name: "hash_X", Args: []string{"%#X\\n", "42"}},
		{Name: "left_align_s", Args: []string{"%-10s|\\n", "hi"}},
		{Name: "left_align_c", Args: []string{"%-5c|\\n", "A"}},
		{Name: "zero_pad_f", Args: []string{"%010.2f\\n", "3.14"}},
		{Name: "plus_f", Args: []string{"%+f\\n", "3.14"}},
		{Name: "space_f", Args: []string{"% f\\n", "3.14"}},
		{Name: "hash_f", Args: []string{"%#.0f\\n", "3.0"}},
		{Name: "hash_g", Args: []string{"%#g\\n", "100.0"}},
		{Name: "hash_e", Args: []string{"%#.0e\\n", "3.14"}},
		{Name: "combined_flags_d", Args: []string{"%-+10d|\\n", "42"}},
		{Name: "combined_zero_plus_d", Args: []string{"%+010d\\n", "42"}},

		// R2.3: star width and precision
		{Name: "star_width", Args: []string{"%*d\\n", "10", "42"}},
		{Name: "star_precision", Args: []string{"%.*f\\n", "2", "3.14159"}},
		{Name: "star_width_precision", Args: []string{"%*.*f\\n", "10", "2", "3.14159"}},
		{Name: "star_neg_width", Args: []string{"%*d|\\n", "-10", "42"}},
		{Name: "star_width_s", Args: []string{"%*s\\n", "10", "hi"}},
		{Name: "star_precision_s", Args: []string{"%.*s\\n", "3", "hello"}},
		{Name: "star_width_precision_s", Args: []string{"%*.*s\\n", "10", "3", "hello"}},
		{Name: "star_width_c", Args: []string{"%*c\\n", "5", "A"}},

		// R2.4: %% literal percent
		{Name: "percent_start", Args: []string{"%%hello"}},
		{Name: "percent_end", Args: []string{"hello%%"}},
		{Name: "percent_mid", Args: []string{"he%%llo"}},
		{Name: "percent_multi", Args: []string{"%%a%%b%%"}},

		// R3.1: escape sequences in format
		{Name: "escape_backslash", Args: []string{"hello\\\\world"}},
		{Name: "escape_octal_A", Args: []string{"\\101"}},
		{Name: "escape_hex_A", Args: []string{"\\x41"}},
		{Name: "escape_bell", Args: []string{"\\a"}},

		// R3.2: argument recycling
		{Name: "recycle_s", Args: []string{"%s\\n", "a", "b", "c"}},
		{Name: "recycle_d", Args: []string{"%d\\n", "1", "2", "3"}},
		{Name: "recycle_multi", Args: []string{"%d %d\\n", "1", "2", "3", "4"}},

		// R3.3: missing arguments (default values)
		{Name: "missing_int_arg", Args: []string{"%d %d\\n", "42"}},
		{Name: "missing_string_arg", Args: []string{"%s %s\\n", "hello"}},
		{Name: "missing_float_arg", Args: []string{"%f %f\\n", "3.14"}},

		// R3.4: character value arguments (quote prefix)
		{Name: "char_value_single", Args: []string{"%d\\n", "'A"}},
		{Name: "char_value_double", Args: []string{"%d\\n", "\"A"}},
		{Name: "char_value_float", Args: []string{"%f\\n", "'A"}},

		// Multiple directives in one format
		{Name: "multi_types", Args: []string{"%d %s %f\\n", "42", "hello", "3.14"}},
		{Name: "no_newline", Args: []string{"%s %s", "hello", "world"}},

		// Combined width/precision/flags edge cases
		{Name: "zero_pad_prec_f", Args: []string{"%010.2f\\n", "3.14159"}},
		{Name: "u_negative", Args: []string{"%u\\n", "-1"}},
		{Name: "x_negative", Args: []string{"%x\\n", "-1"}},
		{Name: "large_width_d", Args: []string{"%20d\\n", "42"}},
		{Name: "string_precision", Args: []string{"%.3s\\n", "hello"}},
		{Name: "precision_zero_d", Args: []string{"%.0d\\n", "0"}},

		// Error cases (R4.2: partial output + exit 1)
		{Name: "non_numeric_d", Args: []string{"%d\\n", "abc"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{stderrNameNormalizer}},
		{Name: "non_numeric_f", Args: []string{"%f\\n", "abc"}, ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{stderrNameNormalizer}},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
