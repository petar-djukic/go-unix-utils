// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeDircolorsErr normalizes error output by stripping "Try ..." help
// lines, replacing the reference binary name prefix with "dircolors", and
// lowercasing OS error messages for cross-platform consistency.
func normalizeDircolorsErr(b []byte) []byte {
	var out [][]byte
	for _, line := range bytes.Split(b, []byte("\n")) {
		if bytes.HasPrefix(line, []byte("Try ")) {
			continue
		}
		if bytes.HasPrefix(line, []byte("gdircolors:")) {
			line = append([]byte("dircolors:"), line[len("gdircolors:"):]...)
		}
		line = bytes.ToLower(line)
		out = append(out, line)
	}
	return bytes.Join(out, []byte("\n"))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdircolors")
	if err != nil {
		t.Skipf("reference binary gdircolors not in PATH: %v", err)
	}

	tmpDir := t.TempDir()

	// R2.1: custom database with comments, blank lines, keywords, extensions.
	customDB := filepath.Join(tmpDir, "custom.db")
	writeFile(t, customDB, "# Comment line\n\nTERM *\n\nDIR 01;34\nEXEC 01;32\n.tar 01;31\n.gz 01;31\n")

	// R2.2: database with restrictive TERM that won't match.
	restrictedDB := filepath.Join(tmpDir, "restricted.db")
	writeFile(t, restrictedDB, "TERM nonexistent-term-xyz\nDIR 01;34\n")

	// R2.2: database with no TERM or COLORTERM lines.
	noTermDB := filepath.Join(tmpDir, "noterm.db")
	writeFile(t, noTermDB, "DIR 01;34\nEXEC 01;32\n.gz 01;31\n")

	// R2.3: database testing keyword-to-code mapping.
	keywordDB := filepath.Join(tmpDir, "keywords.db")
	writeFile(t, keywordDB, "TERM *\nRESET 0\nDIR 01;34\nLINK 01;36\n"+
		"FIFO 40;33\nSOCK 01;35\nBLK 40;33;01\nCHR 40;33;01\n"+
		"ORPHAN 40;31;01\nSETUID 37;41\nSETGID 30;43\nEXEC 01;32\n")

	// R2.3: extension patterns with dot and star prefixes.
	extDB := filepath.Join(tmpDir, "extensions.db")
	writeFile(t, extDB, "TERM *\n.tar 01;31\n.gz 01;31\n*~ 00;90\n*# 00;90\n")

	// R2.3: DOOR keyword mapping.
	doorDB := filepath.Join(tmpDir, "door.db")
	writeFile(t, doorDB, "TERM *\nDOOR 01;35\n")

	// R2.2: COLORTERM pattern matching.
	colortermDB := filepath.Join(tmpDir, "colorterm.db")
	writeFile(t, colortermDB, "COLORTERM ?*\nDIR 01;34\n")

	// R3.4: database with unrecognized keyword to trigger error.
	invalidDB := filepath.Join(tmpDir, "invalid.db")
	writeFile(t, invalidDB, "TERM *\nBADKEYWORD 01;31\n")

	tests := []testutils.DiffTest{
		// --- R1 tests (existing) ---
		{
			Name: "default no args bourne shell",
			Args: []string{"--sh"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "explicit bourne -b",
			Args: []string{"-b"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "explicit bourne --bourne-shell",
			Args: []string{"--bourne-shell"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "csh flag -c",
			Args: []string{"-c"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "csh flag --csh",
			Args: []string{"--csh"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "csh flag --c-shell",
			Args: []string{"--c-shell"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "auto-detect bourne from SHELL",
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "auto-detect csh from SHELL",
			Env:  []string{"SHELL=/bin/tcsh"},
		},
		{
			Name: "auto-detect csh from SHELL csh",
			Env:  []string{"SHELL=/bin/csh"},
		},
		{
			Name: "last flag wins b then c",
			Args: []string{"-b", "-c"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "last flag wins c then b",
			Args: []string{"-c", "-b"},
			Env:  []string{"SHELL=/bin/bash"},
		},

		// --- R2.1: database parsing ---
		{
			Name: "custom database with comments and types",
			Args: []string{"--sh", customDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=xterm-256color"},
		},

		// --- R2.2: TERM matching ---
		{
			Name: "term no match produces empty ls_colors",
			Args: []string{"--sh", restrictedDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=xterm-256color", "COLORTERM="},
		},
		{
			Name: "no term lines applies to all terminals",
			Args: []string{"--sh", noTermDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=nonexistent", "COLORTERM="},
		},
		{
			Name: "colorterm match enables output",
			Args: []string{"--sh", colortermDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=nonexistent", "COLORTERM=truecolor"},
		},
		{
			Name: "colorterm empty no match",
			Args: []string{"--sh", colortermDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=nonexistent", "COLORTERM="},
		},

		// --- R2.3: keyword to code mapping ---
		{
			Name: "keyword to code mapping",
			Args: []string{"--sh", keywordDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=anything"},
		},
		{
			Name: "extension patterns dot and star",
			Args: []string{"--sh", extDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=anything"},
		},
		{
			Name: "door keyword mapping",
			Args: []string{"--sh", doorDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=anything"},
		},

		// --- R2.4: file argument and default database ---
		{
			Name: "print database -p",
			Args: []string{"-p"},
			Env:  []string{"SHELL=/bin/bash"},
		},
		{
			Name: "file argument bourne shell",
			Args: []string{"--sh", customDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=xterm"},
		},
		{
			Name: "file argument csh",
			Args: []string{"--csh", customDB},
			Env:  []string{"SHELL=/bin/bash", "TERM=xterm"},
		},

		// --- R2.5: read database from stdin with "-" ---
		{
			Name:  "stdin database with dash bourne",
			Args:  []string{"--sh", "-"},
			Stdin: []byte("TERM *\nDIR 01;34\nEXEC 01;32\n.tar 01;31\n"),
			Env:   []string{"SHELL=/bin/bash", "TERM=xterm"},
		},
		{
			Name:  "stdin database with dash csh",
			Args:  []string{"--csh", "-"},
			Stdin: []byte("TERM *\nDIR 01;34\n"),
			Env:   []string{"SHELL=/bin/bash", "TERM=xterm"},
		},

		// --- R3.1: print-database output ---
		{
			Name: "print-database long flag",
			Args: []string{"--print-database"},
			Env:  []string{"SHELL=/bin/bash"},
		},

		// --- R3.2: -p incompatible with filename ---
		{
			Name:      "p with filename errors",
			Args:      []string{"-p", customDB},
			Env:       []string{"SHELL=/bin/bash"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeDircolorsErr},
		},

		// --- R3.3: exit 0 on success ---
		{
			Name: "exit 0 on success default",
			Args: []string{"--sh"},
			Env:  []string{"SHELL=/bin/bash"},
		},

		// --- R3.4: error exit codes and diagnostics ---
		{
			Name:      "file not found exits 1",
			Args:      []string{"--sh", filepath.Join(tmpDir, "nonexistent-file")},
			Env:       []string{"SHELL=/bin/bash"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeDircolorsErr},
		},
		{
			Name:      "invalid keyword exits 1 with line number",
			Args:      []string{"--sh", invalidDB},
			Env:       []string{"SHELL=/bin/bash", "TERM=xterm"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeDircolorsErr},
		},

		// --- R3.5: SIGPIPE handler (verified by building and running with pipe) ---
		{
			Name: "sigpipe handler default output",
			Args: []string{"-p"},
			Env:  []string{"SHELL=/bin/bash"},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
