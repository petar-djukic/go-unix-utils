// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd022-nl R1.1–R1.4, R2.1–R2.4, R3.1–R3.4, R4.1–R4.4
package main

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for nl.
const refBinaryName = "gnl"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// D3: LC_ALL=C for all tests.
	env := []string{"LC_ALL=C"}

	tests := []testutils.DiffTest{
		// --- R1.1, R1.2: Default line numbering ---
		{
			Name:  "R1.1 default numbering non-empty lines",
			Args:  []string{},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "R1.2 empty lines not numbered",
			Args:  []string{},
			Stdin: []byte("a\n\nb\n"),
			Env:   env,
		},
		{
			Name:  "R1.3 stdin dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\nworld\n"),
			Env:   env,
		},

		// --- R2.1: -b STYLE body numbering ---
		{
			Name:  "R2.1 body style a numbers all lines",
			Args:  []string{"-ba"},
			Stdin: []byte("a\n\nb\n"),
			Env:   env,
		},
		{
			Name:  "R2.1 body style a attached",
			Args:  []string{"-b", "a"},
			Stdin: []byte("x\n\ny\n"),
			Env:   env,
		},
		{
			Name:  "R2.1 body style t explicit",
			Args:  []string{"-bt"},
			Stdin: []byte("a\n\nb\n"),
			Env:   env,
		},
		{
			Name:  "R2.1 body style n numbers no lines",
			Args:  []string{"-bn"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "R2.1 body style pRE regex match",
			Args:  []string{"-bp^[0-9]"},
			Stdin: []byte("1first\nsecond\n3third\n"),
			Env:   env,
		},
		{
			Name:  "R2.1 body style pRE no match",
			Args:  []string{"-bp^ZZZ"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},

		// --- R2.2: -h STYLE header numbering ---
		{
			Name:  "R2.2 header style a parsed",
			Args:  []string{"-ha"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R2.2 header style n default",
			Args:  []string{"-hn"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},

		// --- R2.3: -f STYLE footer numbering ---
		{
			Name:  "R2.3 footer style a parsed",
			Args:  []string{"-fa"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R2.3 footer style n default",
			Args:  []string{"-fn"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},

		// --- R2.4: style n passes through with no number ---
		{
			Name:  "R2.4 style n no number no separator",
			Args:  []string{"-bn"},
			Stdin: []byte("hello\nworld\n"),
			Env:   env,
		},
		{
			Name:  "R2.4 style n with empty lines",
			Args:  []string{"-bn"},
			Stdin: []byte("a\n\nb\n"),
			Env:   env,
		},

		// --- Combined flags ---
		{
			Name:  "combined body a with header n",
			Args:  []string{"-ba", "-hn"},
			Stdin: []byte("line1\n\nline3\n"),
			Env:   env,
		},
		{
			Name:  "body regex with footer style",
			Args:  []string{"-bp^x", "-fn"},
			Stdin: []byte("x1\ny2\nx3\n"),
			Env:   env,
		},

		// --- R3.1: -n FORMAT line number format ---
		{
			Name:  "R3.1 format rn default",
			Args:  []string{"-ba", "-n", "rn"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "R3.1 format ln left-justified",
			Args:  []string{"-ba", "-n", "ln"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "R3.1 format ln attached",
			Args:  []string{"-ba", "-nln"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R3.1 format rz leading zeros",
			Args:  []string{"-ba", "-n", "rz"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "R3.1 format rz attached",
			Args:  []string{"-ba", "-nrz"},
			Stdin: []byte("x\ny\n"),
			Env:   env,
		},

		// --- R3.2: -w N field width ---
		{
			Name:  "R3.2 width 3",
			Args:  []string{"-ba", "-w", "3"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R3.2 width 10",
			Args:  []string{"-ba", "-w", "10"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R3.2 width attached",
			Args:  []string{"-ba", "-w3"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R3.2 width with rz format",
			Args:  []string{"-ba", "-nrz", "-w", "8"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},

		// --- R3.3: -s SEP separator ---
		{
			Name:  "R3.3 separator colon-space",
			Args:  []string{"-ba", "-s", ": "},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R3.3 separator empty",
			Args:  []string{"-ba", "-s", ""},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R3.3 separator multi-char",
			Args:  []string{"-ba", "-s", " | "},
			Stdin: []byte("x\ny\n"),
			Env:   env,
		},

		// --- R3.4: -v N start value and -i N increment ---
		{
			Name:  "R3.4 start at 10",
			Args:  []string{"-ba", "-v", "10"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "R3.4 start at 10 attached",
			Args:  []string{"-ba", "-v10"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R3.4 increment 5",
			Args:  []string{"-ba", "-i", "5"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "R3.4 increment attached",
			Args:  []string{"-ba", "-i5"},
			Stdin: []byte("a\nb\n"),
			Env:   env,
		},
		{
			Name:  "R3.4 start 10 increment 5",
			Args:  []string{"-ba", "-v", "10", "-i", "5"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "R3.4 start and increment with non-empty only",
			Args:  []string{"-v", "100", "-i", "10"},
			Stdin: []byte("a\n\nb\n"),
			Env:   env,
		},

		// --- Combined R3.1-R3.4 ---
		{
			Name:  "combined ln width 3 separator colon start 10 inc 5",
			Args:  []string{"-ba", "-n", "ln", "-w", "3", "-s", ": ", "-v", "10", "-i", "5"},
			Stdin: []byte("a\nb\nc\n"),
			Env:   env,
		},
		{
			Name:  "combined rz width 8 with empty lines",
			Args:  []string{"-nrz", "-w8", "-v100", "-i10"},
			Stdin: []byte("a\n\nb\n"),
			Env:   env,
		},

		// --- R4.1: Section delimiters ---
		{
			Name:  "R4.1 header body footer delimiters",
			Args:  []string{"-ba", "-ha", "-fa"},
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n\\:\nfooter line\n"),
			Env:   env,
		},
		{
			Name:  "R4.1 delimiter lines not in output",
			Args:  []string{"-ba"},
			Stdin: []byte("before\n\\:\\:\\:\nafter header\n\\:\\:\nafter body\n\\:\nafter footer\n"),
			Env:   env,
		},
		{
			Name:  "R4.1 body delimiter only",
			Args:  []string{"-ba"},
			Stdin: []byte("line1\n\\:\\:\nline2\n"),
			Env:   env,
		},
		{
			Name:  "R4.1 footer delimiter only",
			Args:  []string{"-ba", "-fa"},
			Stdin: []byte("line1\n\\:\nfooter\n"),
			Env:   env,
		},
		{
			Name:  "R4.1 header with default styles",
			Args:  []string{},
			Stdin: []byte("\\:\\:\\:\nheader line\n\\:\\:\nbody line\n\\:\nfooter line\n"),
			Env:   env,
		},
		{
			Name:  "R4.1 header style a numbers header lines",
			Args:  []string{"-ha"},
			Stdin: []byte("\\:\\:\\:\nheader1\nheader2\n\\:\\:\nbody1\n"),
			Env:   env,
		},

		// --- R4.2: Counter reset on header delimiter ---
		{
			Name:  "R4.2 counter resets on header",
			Args:  []string{"-ba"},
			Stdin: []byte("a\nb\n\\:\\:\\:\nc\nd\n"),
			Env:   env,
		},
		{
			Name:  "R4.2 counter resets to -v value",
			Args:  []string{"-ba", "-v", "10"},
			Stdin: []byte("a\nb\n\\:\\:\\:\nc\nd\n"),
			Env:   env,
		},
		{
			Name:  "R4.2 multiple page resets",
			Args:  []string{"-ba"},
			Stdin: []byte("a\n\\:\\:\\:\nb\n\\:\\:\\:\nc\n"),
			Env:   env,
		},

		// --- R4.3: -p suppresses counter reset ---
		{
			Name:  "R4.3 -p no reset on header",
			Args:  []string{"-ba", "-p"},
			Stdin: []byte("a\nb\n\\:\\:\\:\nc\nd\n"),
			Env:   env,
		},
		{
			Name:  "R4.3 -p with -v start",
			Args:  []string{"-ba", "-p", "-v", "10"},
			Stdin: []byte("a\nb\n\\:\\:\\:\nc\nd\n"),
			Env:   env,
		},
		{
			Name:  "R4.3 -p multiple pages",
			Args:  []string{"-ba", "-p"},
			Stdin: []byte("a\n\\:\\:\\:\nb\n\\:\\:\\:\nc\n"),
			Env:   env,
		},

		// --- R4.4: -l N blank line grouping ---
		{
			Name:  "R4.4 -l1 default with -ba",
			Args:  []string{"-ba", "-l", "1"},
			Stdin: []byte("a\n\n\nb\n"),
			Env:   env,
		},
		{
			Name:  "R4.4 -l2 with -ba",
			Args:  []string{"-ba", "-l", "2"},
			Stdin: []byte("a\n\n\nb\n"),
			Env:   env,
		},
		{
			Name:  "R4.4 -l2 with -ba three blanks",
			Args:  []string{"-ba", "-l", "2"},
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   env,
		},
		{
			Name:  "R4.4 -l2 with -ba four blanks",
			Args:  []string{"-ba", "-l", "2"},
			Stdin: []byte("a\n\n\n\n\nb\n"),
			Env:   env,
		},
		{
			Name:  "R4.4 -l3 with -ba",
			Args:  []string{"-ba", "-l", "3"},
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   env,
		},
		{
			Name:  "R4.4 -l2 attached",
			Args:  []string{"-ba", "-l2"},
			Stdin: []byte("a\n\n\nb\n"),
			Env:   env,
		},
		{
			Name:  "R4.4 -l with default body style t",
			Args:  []string{"-l", "2"},
			Stdin: []byte("a\n\n\nb\n"),
			Env:   env,
		},

		// --- Combined R4 ---
		{
			Name:  "R4 combined sections with -p and -l",
			Args:  []string{"-ba", "-ha", "-fa", "-p", "-l", "2"},
			Stdin: []byte("a\n\\:\\:\\:\nb\n\n\nc\n\\:\\:\nd\n"),
			Env:   env,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
