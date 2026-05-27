// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main_test

import (
	"os/exec"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	refBin, err := exec.LookPath("gdircolors")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		{
			Name: "bourne shell flag -b",
			Args: []string{"-b"},
		},
		{
			Name: "bourne shell flag --sh",
			Args: []string{"--sh"},
		},
		{
			Name: "bourne shell flag --bourne-shell",
			Args: []string{"--bourne-shell"},
		},
		{
			Name: "c shell flag -c",
			Args: []string{"-c"},
		},
		{
			Name: "c shell flag --csh",
			Args: []string{"--csh"},
		},
		{
			Name: "c shell flag --c-shell",
			Args: []string{"--c-shell"},
		},
		{
			Name: "auto detect bourne shell",
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "auto detect c shell from tcsh",
			Env:  []string{"SHELL=/bin/tcsh"},
		},
		{
			Name: "auto detect c shell from csh",
			Env:  []string{"SHELL=/bin/csh"},
		},
		{
			Name: "auto detect bourne shell from zsh",
			Env:  []string{"SHELL=/bin/zsh"},
		},
		{
			Name: "last flag wins b then c",
			Args: []string{"-b", "-c"},
		},
		{
			Name: "last flag wins c then b",
			Args: []string{"-c", "-b"},
		},
		{
			Name: "last flag wins long then short",
			Args: []string{"--sh", "-c"},
		},
		{
			Name: "last flag wins short then long",
			Args: []string{"-c", "--bourne-shell"},
		},
		{
			Name: "print database",
			Args: []string{"-p"},
		},
		{
			Name: "print database long flag",
			Args: []string{"--print-database"},
		},
		{
			Name:      "print database with extra operand",
			Args:      []string{"-p", "/dev/null"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		{
			Name:  "custom database from stdin",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("TERM xterm*\nDIR 01;34\n.tar 01;31\n"),
		},
		{
			Name:  "empty database from stdin",
			Args:  []string{"--sh", "-"},
			Stdin: []byte(""),
		},
		{
			Name:  "database with only comments",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("# just a comment\n"),
		},
		{
			Name:  "database with no TERM lines applies to all",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("DIR 01;34\n"),
		},
		{
			Name:      "invalid database missing second token",
			Args:      []string{"--sh", "-"},
			Stdin:     []byte("DIR\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
