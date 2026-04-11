// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/dircolors against gdircolors.
// Implements srd109 AC1–AC6.
package main

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
		t.Skip("reference binary gdircolors not in PATH")
	}

	// Write a custom database file for file-argument tests.
	customDB := filepath.Join(t.TempDir(), "custom.db")
	writeCustomDB(t, customDB)

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

// writeCustomDB creates a small custom database file for testing.
func writeCustomDB(t *testing.T, path string) {
	t.Helper()
	data := "TERM xterm*\nDIR 01;34\nEXEC 01;32\n.tar 01;31\n"
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("failed to write custom db: %v", err)
	}
}
