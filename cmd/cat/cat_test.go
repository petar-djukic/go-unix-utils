// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat covering prd006-cat R1-R4.
// Verifies Go cat against Homebrew GNU gcat (coreutils).
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// refBinaryName is the Homebrew GNU reference binary for cat.
const refBinaryName = "gcat"

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinaryName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinaryName, err)
	}

	// Create fixture files in a shared temp directory for file-based tests.
	fixtureDir := t.TempDir()
	writeFixture(t, fixtureDir, "file1.txt", "hello\nworld\n")
	writeFixture(t, fixtureDir, "file2.txt", "aaa\n")
	writeFixture(t, fixtureDir, "file3.txt", "bbb\n")
	writeFixture(t, fixtureDir, "blanks.txt", "a\n\n\n\nb\n")
	writeFixture(t, fixtureDir, "real.txt", "data\n")
	writeFixture(t, fixtureDir, "tabs.txt", "col1\tcol2\tcol3\n")

	nonexistentPath := filepath.Join(fixtureDir, "nonexistent.txt")
	_ = nonexistentPath // used in args below

	tests := []testutils.DiffTest{
		// R1: File concatenation
		{
			Name:    "default_passthrough",
			Args:    []string{filepath.Join(fixtureDir, "file1.txt")},
			WorkDir: fixtureDir,
		},
		{
			Name:  "stdin_no_args",
			Stdin: []byte("from stdin\n"),
		},
		{
			Name:  "stdin_dash",
			Args:  []string{"-"},
			Stdin: []byte("from stdin\n"),
		},
		{
			Name: "multiple_files",
			Args: []string{
				filepath.Join(fixtureDir, "file2.txt"),
				filepath.Join(fixtureDir, "file3.txt"),
			},
			WorkDir: fixtureDir,
		},
		{
			Name:     "missing_file",
			Args:     []string{filepath.Join(fixtureDir, "nonexistent.txt")},
			ExitCode: 1,
		},
		{
			Name: "missing_and_valid_file",
			Args: []string{
				filepath.Join(fixtureDir, "nonexistent.txt"),
				filepath.Join(fixtureDir, "real.txt"),
			},
			ExitCode: 1,
		},
		{
			Name:  "binary_passthrough",
			Stdin: allBytes(),
		},

		// R2: Line numbering
		{
			Name:  "line_numbering_n",
			Args:  []string{"-n"},
			Stdin: []byte("alpha\n\nbeta\n"),
		},
		{
			Name:  "line_numbering_b",
			Args:  []string{"-b"},
			Stdin: []byte("first\n\n\nsecond\n"),
		},
		{
			Name:  "n_and_b_combined",
			Args:  []string{"-n", "-b"},
			Stdin: []byte("line1\n\nline2\n"),
		},

		// R3: Squeeze blank lines
		{
			Name:  "squeeze_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		{
			Name:  "squeeze_with_n",
			Args:  []string{"-n", "-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		{
			Name:  "squeeze_with_b",
			Args:  []string{"-b", "-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},

		// R4: Special character display
		{
			Name:  "show_nonprinting_v",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x09, 0x1b, 0x7f, 0x80, 0xff},
		},
		{
			Name:  "show_ends_E",
			Args:  []string{"-E"},
			Stdin: []byte("line one\nline two\n"),
		},
		{
			Name:  "show_tabs_T",
			Args:  []string{"-T"},
			Stdin: []byte("col1\tcol2\tcol3\n"),
		},
		{
			Name:  "show_all_A",
			Args:  []string{"-A"},
			Stdin: []byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', '\n'},
		},
		{
			Name:  "flag_e_implies_vE",
			Args:  []string{"-e"},
			Stdin: []byte{0x01, 'h', 'e', 'l', 'l', 'o', '\n'},
		},
		{
			Name:  "flag_t_implies_vT",
			Args:  []string{"-t"},
			Stdin: []byte{0x01, 0x09, 'h', 'e', 'l', 'l', 'o', '\n'},
		},
		{
			Name:  "flag_u_accepted",
			Args:  []string{"-u"},
			Stdin: []byte("test\n"),
		},

		// Flag combinations
		{
			Name:  "vET_explicit",
			Args:  []string{"-v", "-E", "-T"},
			Stdin: []byte{0x01, 0x09, 'h', 'i', '\n'},
		},
		{
			Name:  "squeeze_with_n_and_v",
			Args:  []string{"-s", "-n", "-v"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		{
			Name:  "squeeze_with_b_and_E",
			Args:  []string{"-s", "-b", "-E"},
			Stdin: []byte("x\n\n\n\ny\n"),
		},
		{
			Name:  "all_flags_combined",
			Args:  []string{"-A", "-n", "-s"},
			Stdin: []byte{0x01, '\t', 'a', '\n', '\n', '\n', '\n', 'b', '\n'},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeFixture creates a file in dir with the given name and content.
func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writing fixture %s: %v", name, err)
	}
}

// allBytes returns a 256-byte slice containing every byte value 0x00-0xFF.
func allBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}
