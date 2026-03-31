// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/printf against GNU gprintf.
// Tests prd073-printf R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4.
package main

import (
	"bytes"
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normProgName normalizes the program name prefix in stderr so that
// "gprintf:" and "printf:" both become "printf:" for comparison.
func normProgName(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("gprintf:"), []byte("printf:"))
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gprintf")
	if err != nil {
		t.Skip("reference binary gprintf not in PATH")
	}
	errNorm := []testutils.NormalizeFunc{normProgName}
	tests := []testutils.DiffTest{
		// R1.1: format string interpretation
		{
			Name: "literal text no newline",
			Args: []string{"hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "newline escape in format",
			Args: []string{"hello\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "tab escape in format",
			Args: []string{"A\\tB\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "mixed literal and specifier",
			Args: []string{"num=%d\\n", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "literal percent",
			Args: []string{"100%%\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: integer conversion specifiers
		{
			Name: "percent d positive",
			Args: []string{"%d\\n", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent d negative",
			Args: []string{"%d\\n", "-7"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent d zero",
			Args: []string{"%d\\n", "0"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent i",
			Args: []string{"%i\\n", "99"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent o octal",
			Args: []string{"%o\\n", "255"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent u unsigned",
			Args: []string{"%u\\n", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent x lowercase hex",
			Args: []string{"%x\\n", "255"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent X uppercase hex",
			Args: []string{"%X\\n", "255"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.3: floating-point conversion specifiers
		{
			Name: "percent f",
			Args: []string{"%f\\n", "3.14"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent e scientific",
			Args: []string{"%e\\n", "3.14"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent g general",
			Args: []string{"%g\\n", "3.14"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent g large number",
			Args: []string{"%g\\n", "123456.789"},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.4: string, character, and %b specifiers
		{
			Name: "percent s string",
			Args: []string{"%s\\n", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent c character",
			Args: []string{"%c\\n", "A"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent b simple",
			Args: []string{"%b\\n", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent b with newline escape",
			Args: []string{"%b", "hello\\nworld"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "multiple specifiers",
			Args: []string{"%s is %d years old\\n", "Alice", "30"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.1: width and precision
		{
			Name: "width integer",
			Args: []string{"%10d\\n", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "precision float",
			Args: []string{"%.5f\\n", "3.14"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "width and precision float",
			Args: []string{"%10.3f\\n", "3.14159"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "precision string truncation",
			Args: []string{"%.3s\\n", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "width string",
			Args: []string{"%10s\\n", "hi"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.2: flag characters
		{
			Name: "flag left align",
			Args: []string{"%-10d|\\n", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag plus sign",
			Args: []string{"%+d\\n", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag space before positive",
			Args: []string{"% d\\n", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag zero pad",
			Args: []string{"%05d\\n", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag alternate hex",
			Args: []string{"%#x\\n", "255"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "flag alternate octal",
			Args: []string{"%#o\\n", "255"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "combined zero pad width precision",
			Args: []string{"%010.2f\\n", "3.14159"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.3: star width and precision
		{
			Name: "star width integer",
			Args: []string{"%*d\\n", "10", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "star precision float",
			Args: []string{"%.*f\\n", "2", "3.14159"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "star width and precision float",
			Args: []string{"%*.*f\\n", "10", "2", "3.14159"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "star negative width left align",
			Args: []string{"%*d|\\n", "-10", "42"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "star width string",
			Args: []string{"%*s\\n", "15", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R2.4: literal percent
		{
			Name: "double percent literal",
			Args: []string{"%%\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent in mixed format",
			Args: []string{"%d%% of %s\\n", "50", "total"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.1: escape sequences in FORMAT
		{
			Name: "escape octal",
			Args: []string{"\\101\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape hex",
			Args: []string{"\\x41\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape unicode u",
			Args: []string{"\\u0041\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape unicode U",
			Args: []string{"\\U00000041\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape backslash literal",
			Args: []string{"\\\\\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "escape alert bell",
			Args: []string{"\\a"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.2: argument recycling
		{
			Name: "recycle format one arg",
			Args: []string{"%s\\n", "a", "b", "c"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "recycle format two args",
			Args: []string{"%s=%d\\n", "x", "1", "y", "2"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "recycle single char format",
			Args: []string{"%c\\n", "A", "B", "C"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.3: missing arguments default to 0 or empty
		{
			Name: "missing int arg defaults zero",
			Args: []string{"%d\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "missing float arg defaults zero",
			Args: []string{"%f\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "missing string arg defaults empty",
			Args: []string{"%s\\n"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "partial missing args",
			Args: []string{"%s %d\\n", "hello"},
			Env:  []string{"LC_ALL=C"},
		},
		// R3.4: quote prefix character values
		{
			Name: "single quote A gives 65",
			Args: []string{"%d\\n", "'A"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "double quote A gives 65",
			Args: []string{"%d\\n", "\"A"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "quote prefix in float context",
			Args: []string{"%f\\n", "'A"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "quote prefix zero char",
			Args: []string{"%d\\n", "'0"},
			Env:  []string{"LC_ALL=C"},
		},
		// R4.1: exit 0 on success
		{
			Name:     "exit zero on success",
			Args:     []string{"%d\\n", "42"},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		{
			Name:     "exit zero empty format",
			Args:     []string{""},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R4.2: exit 1 on numeric conversion error, partial output preserved
		{
			Name:      "exit one non-numeric for d",
			Args:      []string{"%d\\n", "abc"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "exit one non-numeric for f",
			Args:      []string{"%f\\n", "notanumber"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		{
			Name:      "partial output before error",
			Args:      []string{"ok %d\\n", "notnum"},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: errNorm,
		},
		// R4.4: additional coverage for comprehensive test suite
		{
			Name: "percent E uppercase scientific",
			Args: []string{"%E\\n", "3.14"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent G uppercase general",
			Args: []string{"%G\\n", "3.14"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent F uppercase float",
			Args: []string{"%F\\n", "3.14"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent b with octal escape",
			Args: []string{"%b\\n", "\\0101"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent b with hex escape",
			Args: []string{"%b\\n", "\\x41"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "percent b with c stop",
			Args: []string{"%b", "hello\\cworld"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "hex integer input",
			Args: []string{"%d\\n", "0xff"},
			Env:  []string{"LC_ALL=C"},
		},
		{
			Name: "octal integer input",
			Args: []string{"%d\\n", "077"},
			Env:  []string{"LC_ALL=C"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
