// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/dircolors against GNU gdircolors.
// Covers prd109-dircolors R1.1 (Bourne shell output), R1.2 (C shell output),
// R1.3 (shell auto-detection), R1.4 (mutually exclusive -b/-c flags),
// R2.1-R2.5 (database parsing, file argument, stdin via "-"),
// R3.1-R3.5 (TERM glob matching, comments/blanks, all keywords/extensions,
// error handling with filename:line diagnostics, --version/--help).
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdircolors")
	if err != nil {
		t.Skip("reference binary gdircolors not in PATH")
	}
	tests := []testutils.DiffTest{
		// R1.1: Bourne shell output format (-b/--sh/--bourne-shell)
		{
			Name: "sh-flag",
			Args: []string{"--sh"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "b-flag",
			Args: []string{"-b"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "bourne-shell-long-flag",
			Args: []string{"--bourne-shell"},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R1.2: C shell output format (-c/--csh/--c-shell)
		{
			Name: "csh-flag",
			Args: []string{"--csh"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "c-flag",
			Args: []string{"-c"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "c-shell-long-flag",
			Args: []string{"--c-shell"},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R1.3: Auto-detect shell from SHELL env
		{
			Name: "auto-detect-bash",
			Args: []string{},
			Env:  []string{"SHELL=/bin/bash", "TERM=xterm-256color"},
		},
		{
			Name: "auto-detect-csh",
			Args: []string{},
			Env:  []string{"SHELL=/bin/csh", "TERM=xterm-256color"},
		},
		{
			Name: "auto-detect-tcsh",
			Args: []string{},
			Env:  []string{"SHELL=/bin/tcsh", "TERM=xterm-256color"},
		},
		{
			Name: "auto-detect-zsh",
			Args: []string{},
			Env:  []string{"SHELL=/bin/zsh", "TERM=xterm-256color"},
		},
		// R1.4: -b and -c mutually exclusive, last one wins
		{
			Name: "last-wins-b-then-c",
			Args: []string{"-b", "-c"},
			Env:  []string{"TERM=xterm-256color"},
		},
		{
			Name: "last-wins-c-then-b",
			Args: []string{"-c", "-b"},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R3.1: -p/--print-database prints human-readable database
		{
			Name: "print-database-short",
			Args: []string{"-p"},
		},
		{
			Name: "print-database-long",
			Args: []string{"--print-database"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCustomDB tests custom database file parsing, TERM matching,
// comments/blank lines, and stdin input.
func TestDiffCustomDB(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdircolors")
	if err != nil {
		t.Skip("reference binary gdircolors not in PATH")
	}

	tmpDir := t.TempDir()
	writeTestFile(t, filepath.Join(tmpDir, "simple.db"),
		"TERM xterm*\nDIR 01;34\nEXEC 01;32\n.tar 01;31\n")
	writeTestFile(t, filepath.Join(tmpDir, "comments.db"),
		"# This is a comment\n\nTERM xterm*\n\n# Another comment\nDIR 01;34\n")
	writeTestFile(t, filepath.Join(tmpDir, "allkeywords.db"),
		"TERM xterm*\nNORMAL 00\nFILE 00\nRESET 0\nDIR 01;34\nLINK 01;36\n"+
			"MULTIHARDLINK 00\nFIFO 40;33\nSOCK 01;35\nDOOR 01;35\n"+
			"BLK 40;33;01\nCHR 40;33;01\nORPHAN 40;31;01\nMISSING 00\n"+
			"SETUID 37;41\nSETGID 30;43\nCAPABILITY 00\n"+
			"STICKY_OTHER_WRITABLE 30;42\nOTHER_WRITABLE 34;42\nSTICKY 37;44\n"+
			"EXEC 01;32\n.tar 01;31\n*.gz 01;31\n")
	writeTestFile(t, filepath.Join(tmpDir, "nomatch.db"),
		"TERM vt999\nDIR 01;34\n")
	writeTestFile(t, filepath.Join(tmpDir, "noterm.db"),
		"DIR 01;34\nEXEC 01;32\n.tar 01;31\n")
	writeTestFile(t, filepath.Join(tmpDir, "ignored.db"),
		"TERM xterm*\nCOLOR tstripped\nOPTIONS tstripped\nEIGHTBIT 1\nDIR 01;34\n")
	writeTestFile(t, filepath.Join(tmpDir, "starext.db"),
		"TERM xterm*\n*.tar 01;31\n*.gz 01;31\n*~ 00;90\n")
	writeTestFile(t, filepath.Join(tmpDir, "inline.db"),
		"TERM xterm*\nDIR 01;34 # directory color\nEXEC 01;32 # executables\n")

	tests := []testutils.DiffTest{
		// R2.4: Custom database file with --sh
		{
			Name: "custom-db-sh",
			Args: []string{"--sh", filepath.Join(tmpDir, "simple.db")},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R2.4: Custom database file with --csh
		{
			Name: "custom-db-csh",
			Args: []string{"--csh", filepath.Join(tmpDir, "simple.db")},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R3.2: Comments and blank lines ignored
		{
			Name: "custom-db-comments",
			Args: []string{"--sh", filepath.Join(tmpDir, "comments.db")},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R3.3: All file type keywords supported
		{
			Name: "custom-db-all-keywords",
			Args: []string{"--sh", filepath.Join(tmpDir, "allkeywords.db")},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R2.2/R3.1: TERM non-matching produces empty LS_COLORS
		{
			Name: "custom-db-term-nomatch",
			Args: []string{"--sh", filepath.Join(tmpDir, "nomatch.db")},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R2.2: No TERM lines means colors apply to all terminals
		{
			Name: "custom-db-no-term-lines",
			Args: []string{"--sh", filepath.Join(tmpDir, "noterm.db")},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R3.2: Ignored keywords (COLOR, OPTIONS, EIGHTBIT)
		{
			Name: "custom-db-ignored-keywords",
			Args: []string{"--sh", filepath.Join(tmpDir, "ignored.db")},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R3.3: Extension entries with * prefix and glob patterns
		{
			Name: "custom-db-star-extensions",
			Args: []string{"--sh", filepath.Join(tmpDir, "starext.db")},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R3.2: Inline comments after values
		{
			Name: "custom-db-inline-comments",
			Args: []string{"--sh", filepath.Join(tmpDir, "inline.db")},
			Env:  []string{"TERM=xterm-256color"},
		},
		// R2.5: Read database from stdin via "-"
		{
			Name:  "stdin-db",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("TERM xterm*\nDIR 01;34\nEXEC 01;32\n"),
			Env:   []string{"TERM=xterm-256color"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// normalizeProgramName replaces "gdircolors" with "dircolors" in output
// so that error messages from the reference binary match the Go binary.
func normalizeProgramName(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("gdircolors"), []byte("dircolors"))
	return data
}

// normalizeTryHelp normalizes the "Try '...' --help" line to remove
// full binary paths that differ between Go and GNU binaries.
func normalizeTryHelp(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	for i, line := range lines {
		if !bytes.HasPrefix(line, []byte("Try '")) {
			continue
		}
		if !bytes.HasSuffix(line, []byte("' for more information.")) {
			continue
		}
		lines[i] = []byte("Try 'dircolors --help' for more information.")
	}
	return bytes.Join(lines, []byte("\n"))
}

// TestDiffErrors tests error handling for invalid configuration files.
// R3.4: error diagnostics with filename and line number.
func TestDiffErrors(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdircolors")
	if err != nil {
		t.Skip("reference binary gdircolors not in PATH")
	}

	tmpDir := t.TempDir()
	writeTestFile(t, filepath.Join(tmpDir, "badkeyword.db"),
		"TERM xterm*\nBADKEYWORD 01;34\n")
	writeTestFile(t, filepath.Join(tmpDir, "missing_token.db"),
		"TERM xterm*\nDIR\n")
	writeTestFile(t, filepath.Join(tmpDir, "multiple_errors.db"),
		"TERM xterm*\nBADKEY1 01;34\nDIR 01;34\nBADKEY2 01;35\n")

	norm := []testutils.NormalizeFunc{normalizeProgramName, normalizeTryHelp}

	tests := []testutils.DiffTest{
		// R3.4: Unrecognized keyword produces error with filename:line
		{
			Name:      "error-unrecognized-keyword",
			Args:      []string{"--sh", filepath.Join(tmpDir, "badkeyword.db")},
			Env:       []string{"TERM=xterm-256color"},
			ExitCode:  1,
			Normalize: norm,
		},
		// R3.4: Missing second token produces error with filename:line
		{
			Name:      "error-missing-second-token",
			Args:      []string{"--sh", filepath.Join(tmpDir, "missing_token.db")},
			Env:       []string{"TERM=xterm-256color"},
			ExitCode:  1,
			Normalize: norm,
		},
		// R3.4: Multiple errors are all reported
		{
			Name:      "error-multiple-errors",
			Args:      []string{"--sh", filepath.Join(tmpDir, "multiple_errors.db")},
			Env:       []string{"TERM=xterm-256color"},
			ExitCode:  1,
			Normalize: norm,
		},
		// R3.4: -p with filename gives extra operand error
		{
			Name:      "error-print-db-with-file",
			Args:      []string{"-p", filepath.Join(tmpDir, "badkeyword.db")},
			Env:       []string{"TERM=xterm-256color"},
			ExitCode:  1,
			Normalize: norm,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestVersionFlag verifies --version outputs version info and exits 0.
// R3.5: --version flag support.
func TestVersionFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !bytes.Contains(out, []byte("dircolors")) {
		t.Errorf("--version output missing program name: %q", out)
	}
}

// TestHelpFlag verifies --help outputs usage info and exits 0.
// R3.5: --help flag support.
func TestHelpFlag(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if !bytes.Contains(out, []byte("Usage:")) {
		t.Errorf("--help output missing Usage line: %q", out)
	}
}

// writeTestFile creates a file with the given content for testing.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file %s: %v", path, err)
	}
}
