// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gptx")
	if err != nil {
		t.Skip("reference binary gptx not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:  "basic kwic index from stdin",
			Args:  []string{},
			Stdin: []byte("hello world\n"),
		},
		{
			Name:  "empty input produces no output",
			Args:  []string{},
			Stdin: []byte(""),
		},
		{
			Name:  "single word",
			Args:  []string{},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "three words",
			Args:  []string{},
			Stdin: []byte("a b c\n"),
		},

		// R2.1: width and gap-size flags
		{
			Name:  "width short flag -w",
			Args:  []string{"-w", "50"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "width long flag --width=N",
			Args:  []string{"--width=50"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "width long flag --width N",
			Args:  []string{"--width", "50"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "width attached -wN",
			Args:  []string{"-w50"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "gap-size short flag -g",
			Args:  []string{"-g", "5"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "gap-size long flag --gap-size=N",
			Args:  []string{"--gap-size=5"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "gap-size attached -gN",
			Args:  []string{"-g5"},
			Stdin: []byte("a b c\n"),
		},
		{
			Name:  "width and gap-size combined",
			Args:  []string{"-w", "50", "-g", "5"},
			Stdin: []byte("hello world\n"),
		},

		// R2.2: ignore-case flag
		{
			Name:  "ignore-case short flag -f",
			Args:  []string{"-f"},
			Stdin: []byte("Beta alpha\n"),
		},
		{
			Name:  "ignore-case long flag --ignore-case",
			Args:  []string{"--ignore-case"},
			Stdin: []byte("Beta alpha\n"),
		},
		{
			Name:  "case-sensitive sort (no -f)",
			Args:  []string{},
			Stdin: []byte("Beta alpha\n"),
		},
		{
			Name:  "ignore-case with mixed words",
			Args:  []string{"-f"},
			Stdin: []byte("x Y z\n"),
		},
		{
			Name:  "combined -f and -w flags",
			Args:  []string{"-fw", "50"},
			Stdin: []byte("Beta alpha\n"),
		},

		// R2.3: file and stdin reading
		{
			Name:  "read from explicit stdin dash",
			Args:  []string{"-"},
			Stdin: []byte("hello world\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
