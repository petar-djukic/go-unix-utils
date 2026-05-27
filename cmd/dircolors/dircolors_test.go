// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
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
		{
			Name:  "R2.1 inline comments after value",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("DIR 01;34 # a directory\n.tar 01;31 # archive\n"),
		},
		{
			Name:      "R2.1 unrecognized bare keyword",
			Args:      []string{"--sh", "-"},
			Stdin:     []byte("FOOBAR\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		{
			Name:  "R2.1 unrecognized keyword with value ignored",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("DIR 01;34\nFOOBAR 01;31\n.tar 01;31\n"),
		},
		{
			Name:  "R2.2 TERM lines match current terminal",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("TERM xterm*\nDIR 01;34\n.tar 01;31\n"),
			Env:   []string{"TERM=xterm-256color"},
		},
		{
			Name:  "R2.2 TERM lines do not match current terminal",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("TERM xterm*\nDIR 01;34\n.tar 01;31\n"),
			Env:   []string{"TERM=dumb"},
		},
		{
			Name:  "R2.3 multiple file type keywords",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("DIR 01;34\nLINK 01;36\nEXEC 01;32\nFIFO 40;33\nSOCK 01;35\nBLK 40;33;01\nCHR 40;33;01\nORPHAN 40;31;01\nSETUID 37;41\nSETGID 30;43\nSTICKY 37;44\nOTHER_WRITABLE 34;42\nSTICKY_OTHER_WRITABLE 30;42\nCAPABILITY 00\nMULTIHARDLINK 00\nNORMAL 00\nFILE 00\nRESET 0\nMISSING 00\n"),
		},
		{
			Name:  "R2.3 extension dot prefix becomes star-dot",
			Args:  []string{"--sh", "-"},
			Stdin: []byte(".tar 01;31\n.gz 01;31\n"),
		},
		{
			Name:  "R2.3 extension star-dot prefix preserved",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("*.tar 01;31\n*.gz 01;31\n"),
		},
		{
			Name:  "R2.5 read database from stdin via dash",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("DIR 01;34\nLINK 01;36\n.tar 01;31\n"),
		},
		{
			Name:  "R2.5 stdin empty produces empty LS_COLORS",
			Args:  []string{"--sh", "-"},
			Stdin: []byte(""),
		},
		{
			Name:  "R2.5 stdin with TERM and extensions",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("TERM xterm*\nDIR 01;34\nEXEC 01;32\n.gz 01;31\n"),
		},
		{
			Name: "R3.1 print-database outputs built-in defaults",
			Args: []string{"-p"},
		},
		{
			Name: "R3.1 print-database via long flag",
			Args: []string{"--print-database"},
		},
		{
			Name:      "R3.2 print-database incompatible with file operand",
			Args:      []string{"-p", "/dev/null"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		{
			Name:      "R3.2 print-database incompatible with dash operand",
			Args:      []string{"--print-database", "-"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		{
			Name: "R3.3 exit 0 on successful bourne output",
			Args: []string{"--sh"},
		},
		{
			Name: "R3.3 exit 0 on successful csh output",
			Args: []string{"--csh"},
		},
		{
			Name:  "R3.3 exit 0 on successful stdin parse",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("DIR 01;34\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)

	dbContent := []byte("DIR 01;34\nLINK 01;36\n.tar 01;31\n")
	tmpDir := t.TempDir()
	dbFile := filepath.Join(tmpDir, "colors.db")
	if err := os.WriteFile(dbFile, dbContent, 0644); err != nil {
		t.Fatal(err)
	}

	fileTests := []testutils.DiffTest{
		{
			Name: "R2.4 database from file argument",
			Args: []string{"--sh", dbFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, fileTests)
}
