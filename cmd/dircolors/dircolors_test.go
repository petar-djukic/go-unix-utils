// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/dircolors against gdircolors.
// Implements srd109 AC1–AC6.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// reProgName matches gdircolors with an optional path prefix.
var reProgName = regexp.MustCompile(`(?:/[^ ']*)?gdircolors`)

// normalizeProgramName replaces gdircolors (with optional path) with "dircolors"
// so error messages from both binaries match despite different program names.
func normalizeProgramName(data []byte) []byte {
	return reProgName.ReplaceAll(data, []byte("dircolors"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdircolors")
	if err != nil {
		t.Skip("reference binary gdircolors not in PATH")
	}

	tmpDir := t.TempDir()

	// Custom database files for various test scenarios.
	customDB := filepath.Join(tmpDir, "custom.db")
	writeFile(t, customDB,
		"TERM xterm*\nDIR 01;34\nEXEC 01;32\n.tar 01;31\n")

	// R3.1/R2.2: TERM vt100 only — won't match xterm-256color.
	customDBTermVT := filepath.Join(tmpDir, "term_vt.db")
	writeFile(t, customDBTermVT,
		"TERM vt100\nDIR 01;34\n.tar 01;31\n")

	// R3.1: no TERM lines — applies to all terminals.
	customDBNoTerm := filepath.Join(tmpDir, "no_term.db")
	writeFile(t, customDBNoTerm,
		"DIR 01;34\nEXEC 01;32\n.gz 01;31\n")

	// R3.2: comments and blank lines interspersed.
	customDBComments := filepath.Join(tmpDir, "comments.db")
	writeFile(t, customDBComments,
		"# This is a comment\n"+
			"\n"+
			"# Another comment\n"+
			"TERM xterm*\n"+
			"\n"+
			"# File types below\n"+
			"DIR 01;34\n"+
			"\n"+
			"# Extensions\n"+
			".tar 01;31\n"+
			".gz 01;31\n")

	// R3.3: many keyword types and extensions.
	customDBKeywords := filepath.Join(tmpDir, "keywords.db")
	writeFile(t, customDBKeywords,
		"TERM xterm*\n"+
			"RESET 0\n"+
			"DIR 01;34\n"+
			"LINK 01;36\n"+
			"FIFO 40;33\n"+
			"SOCK 01;35\n"+
			"BLK 40;33;01\n"+
			"CHR 40;33;01\n"+
			"ORPHAN 40;31;01\n"+
			"SETUID 37;41\n"+
			"SETGID 30;43\n"+
			"STICKY 37;44\n"+
			"OTHER_WRITABLE 34;42\n"+
			"STICKY_OTHER_WRITABLE 30;42\n"+
			"EXEC 01;32\n"+
			".tar 01;31\n"+
			".jpg 01;35\n")

	// R3.3: extensions with *. prefix form.
	customDBStarExt := filepath.Join(tmpDir, "star_ext.db")
	writeFile(t, customDBStarExt,
		"TERM xterm*\n"+
			"DIR 01;34\n"+
			"*.tar 01;31\n"+
			"*.gz 01;31\n"+
			"*.jpg 01;35\n")

	// R3.4: database with unrecognized keyword.
	invalidKeywordDB := filepath.Join(tmpDir, "invalid_kw.db")
	writeFile(t, invalidKeywordDB,
		"TERM xterm*\nBADKW 01;31\n")

	// R3.4: database with missing second token.
	malformedDB := filepath.Join(tmpDir, "malformed.db")
	writeFile(t, malformedDB,
		"TERM xterm*\nBADLINE\n")

	// R3.4: database with multiple errors.
	multiErrorDB := filepath.Join(tmpDir, "multi_error.db")
	writeFile(t, multiErrorDB,
		"TERM xterm*\nBADKW1 01;31\nDIR 01;34\nBADKW2 01;35\n")

	// Path to a nonexistent file for file-not-found testing.
	nonexistentDB := filepath.Join(tmpDir, "nonexistent.db")

	tests := []testutils.DiffTest{
		// AC1: Bourne shell output with --sh
		{
			Name: "sh flag",
			Args: []string{"--sh"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// AC1: Bourne shell output with -b
		{
			Name: "b flag",
			Args: []string{"-b"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// AC1: --bourne-shell long form
		{
			Name: "bourne-shell flag",
			Args: []string{"--bourne-shell"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// AC2: C shell output with --csh
		{
			Name: "csh flag",
			Args: []string{"--csh"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// AC2: C shell output with -c
		{
			Name: "c flag",
			Args: []string{"-c"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// AC2: --c-shell long form
		{
			Name: "c-shell flag",
			Args: []string{"--c-shell"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// AC3: print-database with -p
		{
			Name: "print-database short",
			Args: []string{"-p"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// AC3: print-database with --print-database
		{
			Name: "print-database long",
			Args: []string{"--print-database"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// AC5: shell auto-detection with SHELL=/bin/bash
		{
			Name: "auto-detect bash",
			Args: nil,
			Env:  shellEnv("/bin/bash", "xterm-256color", ""),
		},
		// AC5: shell auto-detection with SHELL=/bin/tcsh
		{
			Name: "auto-detect tcsh",
			Args: nil,
			Env:  shellEnv("/bin/tcsh", "xterm-256color", ""),
		},
		// AC5: shell auto-detection with SHELL=/bin/csh
		{
			Name: "auto-detect csh",
			Args: nil,
			Env:  shellEnv("/bin/csh", "xterm-256color", ""),
		},
		// R1.4: last flag wins
		{
			Name: "last flag wins sh",
			Args: []string{"--csh", "--sh"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		{
			Name: "last flag wins csh",
			Args: []string{"--sh", "--csh"},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// TERM matching: non-matching TERM with no COLORTERM
		{
			Name: "non-matching term",
			Args: []string{"--sh"},
			Env:  defaultEnv("dumb", ""),
		},
		// COLORTERM matching: non-matching TERM but valid COLORTERM
		{
			Name: "colorterm match",
			Args: []string{"--sh"},
			Env:  defaultEnv("dumb", "truecolor"),
		},
		// AC4: custom database file with --sh
		{
			Name: "custom db sh",
			Args: []string{"--sh", customDB},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// AC4: custom database file with --csh
		{
			Name: "custom db csh",
			Args: []string{"--csh", customDB},
			Env:  defaultEnv("xterm-256color", ""),
		},

		// --- Tests for R2.5, R3.1–R3.3 ---

		// R2.5: read custom database from stdin via "-"
		{
			Name:  "stdin custom db sh",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("TERM xterm*\nDIR 01;34\nEXEC 01;32\n.tar 01;31\n"),
			Env:   defaultEnv("xterm-256color", ""),
		},
		// R2.5: stdin with C shell output
		{
			Name:  "stdin custom db csh",
			Args:  []string{"--csh", "-"},
			Stdin: []byte("TERM xterm*\nDIR 01;34\n.tar 01;31\n"),
			Env:   defaultEnv("xterm-256color", ""),
		},
		// R3.1: TERM pattern non-match in custom db
		{
			Name: "custom db term nomatch",
			Args: []string{"--sh", customDBTermVT},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// R3.1: no TERM filter — applies to all terminals including dumb
		{
			Name: "custom db no term filter",
			Args: []string{"--sh", customDBNoTerm},
			Env:  defaultEnv("dumb", ""),
		},
		// R3.2: comments and blank lines are ignored in custom db
		{
			Name: "custom db comments blanks",
			Args: []string{"--sh", customDBComments},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// R3.3: many keywords in custom db with --sh
		{
			Name: "custom db keywords sh",
			Args: []string{"--sh", customDBKeywords},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// R3.3: many keywords in custom db with --csh
		{
			Name: "custom db keywords csh",
			Args: []string{"--csh", customDBKeywords},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// R3.3: extensions with *. prefix form
		{
			Name: "custom db star ext",
			Args: []string{"--sh", customDBStarExt},
			Env:  defaultEnv("xterm-256color", ""),
		},
		// R2.5: stdin with no TERM filter
		{
			Name:  "stdin no term filter",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("DIR 01;34\n.tar 01;31\n"),
			Env:   defaultEnv("dumb", ""),
		},

		// --- R3.4-R3.5: error handling and edge cases ---

		// R3.4/R3.5: unrecognized option exits non-zero with diagnostic
		{
			Name:      "unrecognized option",
			Args:      []string{"--invalid-option"},
			Env:       defaultEnv("xterm-256color", ""),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.4/R3.5: nonexistent file exits non-zero with diagnostic
		{
			Name:      "nonexistent file",
			Args:      []string{"--sh", nonexistentDB},
			Env:       defaultEnv("xterm-256color", ""),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.4/R3.5: unrecognized keyword in database
		{
			Name:      "invalid keyword in db",
			Args:      []string{"--sh", invalidKeywordDB},
			Env:       defaultEnv("xterm-256color", ""),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.4/R3.5: malformed line missing second token
		{
			Name:      "malformed db line",
			Args:      []string{"--sh", malformedDB},
			Env:       defaultEnv("xterm-256color", ""),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.4: multiple errors reported for all bad lines
		{
			Name:      "multiple db errors",
			Args:      []string{"--sh", multiErrorDB},
			Env:       defaultEnv("xterm-256color", ""),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R3.5: extra operand exits non-zero
		{
			Name:      "extra operand",
			Args:      []string{"--sh", customDB, customDBNoTerm},
			Env:       defaultEnv("xterm-256color", ""),
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// defaultEnv returns a minimal env with LC_ALL=C, SHELL=/bin/sh, and the given TERM/COLORTERM.
func defaultEnv(term, colorterm string) []string {
	env := []string{
		"LC_ALL=C",
		"SHELL=/bin/sh",
		"TERM=" + term,
	}
	if colorterm != "" {
		env = append(env, "COLORTERM="+colorterm)
	}
	return env
}

// shellEnv returns a minimal env with the specified SHELL, TERM, and COLORTERM.
func shellEnv(shell, term, colorterm string) []string {
	env := []string{
		"LC_ALL=C",
		"SHELL=" + shell,
		"TERM=" + term,
	}
	if colorterm != "" {
		env = append(env, "COLORTERM="+colorterm)
	}
	return env
}

// writeFile creates a file with the given content for testing.
func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
