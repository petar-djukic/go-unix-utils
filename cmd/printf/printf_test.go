// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/printf. Implements srd073-printf R4.3, R4.4.
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gprintf")
	if err != nil {
		t.Skipf("reference binary gprintf not in PATH: %v", err)
	}

	// R4.4: tests cover integer formats, float formats, string, char,
	// backslash-interpreted string, width and precision, all flags,
	// * width/precision, argument recycling, escape sequences, character
	// value arguments, missing arguments, and error cases.
	tests := []testutils.DiffTest{
		// R1.1: literal text, no trailing newline unless \n in format
		{
			Name: "literal text only",
			Args: []string{"hello world"},
		},
		{
			Name: "literal text with newline escape",
			Args: []string{"hello\\n"},
		},
		{
			Name: "format with no specifiers",
			Args: []string{"abc"},
		},
		{
			Name: "mixed literal and specifier",
			Args: []string{"val=%d\\n", "42"},
		},

		// R1.2: integer specifiers %d, %i, %o, %u, %x, %X
		{
			Name: "percent d positive",
			Args: []string{"%d\\n", "42"},
		},
		{
			Name: "percent d negative",
			Args: []string{"%d\\n", "-7"},
		},
		{
			Name: "percent d zero",
			Args: []string{"%d\\n", "0"},
		},
		{
			Name: "percent i",
			Args: []string{"%i\\n", "99"},
		},
		{
			Name: "percent o octal",
			Args: []string{"%o\\n", "255"},
		},
		{
			Name: "percent u unsigned",
			Args: []string{"%u\\n", "42"},
		},
		{
			Name: "percent x lowercase hex",
			Args: []string{"%x\\n", "255"},
		},
		{
			Name: "percent X uppercase hex",
			Args: []string{"%X\\n", "255"},
		},
		{
			Name: "percent d hex input",
			Args: []string{"%d\\n", "0xff"},
		},
		{
			Name: "percent d octal input",
			Args: []string{"%d\\n", "077"},
		},

		// R1.3: floating-point specifiers %f, %e, %g, %F, %E, %G
		{
			Name: "percent f",
			Args: []string{"%f\\n", "3.14"},
		},
		{
			Name: "percent f integer",
			Args: []string{"%f\\n", "42"},
		},
		{
			Name: "percent e scientific",
			Args: []string{"%e\\n", "12345.6789"},
		},
		{
			Name: "percent E uppercase scientific",
			Args: []string{"%E\\n", "12345.6789"},
		},
		{
			Name: "percent g shorter",
			Args: []string{"%g\\n", "100.0"},
		},
		{
			Name: "percent g large",
			Args: []string{"%g\\n", "123456789.0"},
		},
		{
			Name: "percent G uppercase",
			Args: []string{"%G\\n", "12345.6789"},
		},
		{
			Name: "percent F uppercase",
			Args: []string{"%F\\n", "3.14"},
		},
		{
			Name: "percent f negative",
			Args: []string{"%f\\n", "-2.718"},
		},

		// R1.4: %s, %c, %b
		{
			Name: "percent s string",
			Args: []string{"%s\\n", "hello"},
		},
		{
			Name: "percent s empty",
			Args: []string{"%s\\n", ""},
		},
		{
			Name: "percent c character",
			Args: []string{"%c\\n", "A"},
		},
		{
			Name: "percent c from string",
			Args: []string{"%c\\n", "hello"},
		},
		{
			Name: "percent b backslash escapes",
			Args: []string{"%b\\n", "hello\\nworld"},
		},
		{
			Name: "percent b tab escape",
			Args: []string{"%b\\n", "col1\\tcol2"},
		},
		{
			Name: "percent b octal escape",
			Args: []string{"%b", "\\0101"},
		},
		{
			Name: "percent b backslash literal",
			Args: []string{"%b\\n", "a\\\\b"},
		},

		// R1.1 + R1.2 + R1.4 combined
		{
			Name: "mixed d and s",
			Args: []string{"%d %s\\n", "42", "hello"},
		},
		{
			Name: "multiple specifiers",
			Args: []string{"%s is %d\\n", "age", "30"},
		},
	}

	// R4.3: compare Go printf output against gprintf byte-for-byte.
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
