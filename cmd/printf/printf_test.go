// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/printf against GNU gprintf.
// Tests prd073-printf R1.1-R1.4.
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
		t.Skip("reference binary gprintf not in PATH")
	}
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
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
