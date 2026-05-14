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
	refBin, err := exec.LookPath("ghead")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R1.5: last line without newline is still counted
		{
			Name:  "line-no-trailing-newline",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc"),
		},
		{
			Name:  "line-with-trailing-newline",
			Args:  []string{"-n", "3"},
			Stdin: []byte("a\nb\nc\n"),
		},
		{
			Name:  "line-count-exceeds-input",
			Args:  []string{"-n", "100"},
			Stdin: []byte("one\ntwo\n"),
		},
		{
			Name:  "line-no-trailing-newline-negative",
			Args:  []string{"-n", "-1"},
			Stdin: []byte("a\nb\nc"),
		},

		// R2.1: -c NUM byte count
		{
			Name:  "bytes-5",
			Args:  []string{"-c", "5"},
			Stdin: []byte("abcdefghij"),
		},
		{
			Name:  "bytes-exceeds-input",
			Args:  []string{"-c", "100"},
			Stdin: []byte("short"),
		},
		{
			Name:  "bytes-zero",
			Args:  []string{"-c", "0"},
			Stdin: []byte("hello"),
		},

		// R2.1: -c and -n mutual exclusivity (last wins)
		{
			Name:  "bytes-after-lines",
			Args:  []string{"-n", "2", "-c", "5"},
			Stdin: []byte("abcdefghij\nklmnop\n"),
		},
		{
			Name:  "lines-after-bytes",
			Args:  []string{"-c", "5", "-n", "1"},
			Stdin: []byte("abcdefghij\nklmnop\n"),
		},

		// R2.2: negative byte count
		{
			Name:  "bytes-negative",
			Args:  []string{"-c", "-3"},
			Stdin: []byte("abcdefghij"),
		},
		{
			Name:  "bytes-negative-exceeds-input",
			Args:  []string{"-c", "-100"},
			Stdin: []byte("short"),
		},

		// R2.3: multiplier suffixes
		{
			Name:  "bytes-suffix-b",
			Args:  []string{"-c", "1b"},
			Stdin: bytes512(),
		},
		{
			Name:  "bytes-long-form",
			Args:  []string{"--bytes=5"},
			Stdin: []byte("abcdefghij"),
		},
		{
			Name:  "bytes-long-form-negative",
			Args:  []string{"--bytes=-3"},
			Stdin: []byte("abcdefghij"),
		},

		// R2.1: stdin via -
		{
			Name:  "bytes-stdin-dash",
			Args:  []string{"-c", "3", "-"},
			Stdin: []byte("hello"),
		},

		// Error: invalid byte count
		{
			Name:      "bytes-invalid",
			Args:      []string{"-c", "abc"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func bytes512() []byte {
	b := make([]byte, 600)
	for i := range b {
		b[i] = byte('a' + i%26)
	}
	return b
}
