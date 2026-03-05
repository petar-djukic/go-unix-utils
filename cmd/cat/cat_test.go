// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main tests cmd/cat via differential tests against the GNU reference
// binary (gcat from Homebrew coreutils on macOS, or cat on Linux).
//
// Implements: prd006-cat R1-R5
// Test suite: docs/specs/test-suites/test-rel01.1.yaml
// Architecture: docs/ARCHITECTURE.yaml § pkg/testutils
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goCat is the absolute path to the Go cat binary, built by TestMain.
var goCat string

// refCat is the absolute path to the GNU reference binary, resolved in TestMain.
var refCat string

// TestMain builds the Go cat binary once and locates the GNU reference binary
// before running any test. It uses os.Exit(m.Run()) as required by the testing
// package when TestMain is defined.
func TestMain(m *testing.M) {
	var err error
	goCat, err = buildGoCat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build cmd/cat: %v\n", err)
		os.Exit(1)
	}
	refCat = resolveRefCat()
	os.Exit(m.Run())
}

// buildGoCat compiles cmd/cat into bin/cat under the project root and returns
// the absolute path to the binary.
func buildGoCat() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}
	binDir := filepath.Join(cwd, "..", "..", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir bin: %w", err)
	}
	binPath := filepath.Join(binDir, "cat")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return binPath, nil
}

// resolveRefCat locates the GNU cat reference binary. On macOS, Homebrew
// installs it as gcat. On Linux, the system cat is GNU cat.
func resolveRefCat() string {
	primary, fallback := "gcat", "cat"
	if runtime.GOOS == "linux" {
		primary, fallback = "cat", "gcat"
	}
	for _, name := range []string{primary, fallback} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return primary // leave DiffTest.Run to fail with a useful diagnostic
}

// lcEnv returns the environment slice applied to every test case.
// LC_ALL=C is required by ARCHITECTURE.yaml DD6 to eliminate locale divergence.
func lcEnv() []string {
	return []string{"LC_ALL=C"}
}

// writeFile creates path with the given content. It is a test helper that
// registers a cleanup for the containing test.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// allBytes returns a slice containing all 256 byte values in ascending order.
func allBytes() []byte {
	b := make([]byte, 256)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

// TestCat is a table-driven test covering stdin-based cat behaviors for all
// flags defined in prd006-cat R1-R4. File-system-dependent cases are covered
// by separate test functions below.
func TestCat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		stdin      []byte
		wantExit   int
		wantStdout []byte
	}{
		// R1.2: no file arguments — read from stdin.
		{
			name:       "stdin_no_args",
			args:       nil,
			stdin:      []byte("from stdin\n"),
			wantExit:   0,
			wantStdout: []byte("from stdin\n"),
		},
		// R1.2: "-" as a filename reads from stdin.
		{
			name:       "stdin_dash",
			args:       []string{"-"},
			stdin:      []byte("from stdin\n"),
			wantExit:   0,
			wantStdout: []byte("from stdin\n"),
		},
		// R1.4: binary input (all 256 byte values) passes through without corruption.
		{
			name:       "binary_passthrough",
			args:       nil,
			stdin:      allBytes(),
			wantExit:   0,
			wantStdout: allBytes(),
		},
		// R2.1: -n prepends a right-justified 6-wide line number followed by a tab
		// to every output line, including blank lines. Line numbering starts at 1.
		{
			name:       "flag_n",
			args:       []string{"-n"},
			stdin:      []byte("alpha\n\nbeta\n"),
			wantExit:   0,
			wantStdout: []byte("     1\talpha\n     2\t\n     3\tbeta\n"),
		},
		// R2.2, R2.4: -b numbers only non-blank lines; blank lines (a line
		// containing only a newline) receive no prefix.
		{
			name:       "flag_b",
			args:       []string{"-b"},
			stdin:      []byte("first\n\n\nsecond\n"),
			wantExit:   0,
			wantStdout: []byte("     1\tfirst\n\n\n     2\tsecond\n"),
		},
		// R2.3: when -b and -n are both given, -b takes precedence and blank
		// lines are not numbered.
		{
			name:       "flag_bn_b_takes_precedence",
			args:       []string{"-b", "-n"},
			stdin:      []byte("one\n\ntwo\n"),
			wantExit:   0,
			wantStdout: []byte("     1\tone\n\n     2\ttwo\n"),
		},
		// R3.1: -s collapses consecutive blank lines to a single blank line.
		{
			name:       "flag_s",
			args:       []string{"-s"},
			stdin:      []byte("a\n\n\n\nb\n"),
			wantExit:   0,
			wantStdout: []byte("a\n\nb\n"),
		},
		// R3.3, R4.9: -n -s combined — squeeze is applied before numbering so
		// suppressed blank lines do not consume line numbers.
		{
			name:       "flag_ns_combined",
			args:       []string{"-n", "-s"},
			stdin:      []byte("a\n\n\n\nb\n"),
			wantExit:   0,
			wantStdout: []byte("     1\ta\n     2\t\n     3\tb\n"),
		},
		// R4.1, R4.2: -v displays non-printing bytes with caret/M- notation.
		// Tab (0x09) and newline (0x0A) are exempted and pass through unchanged.
		// Input bytes: SOH(0x01) TAB(0x09) ESC(0x1B) DEL(0x7F) 0x80 0xFF
		{
			name:       "flag_v_nonprinting",
			args:       []string{"-v"},
			stdin:      []byte{0x01, 0x09, 0x1B, 0x7F, 0x80, 0xFF},
			wantExit:   0,
			wantStdout: []byte("^A\t^[^?M-^@M-^?"),
		},
		// R4.3: -E appends "$" before each newline in the output.
		{
			name:       "flag_E",
			args:       []string{"-E"},
			stdin:      []byte("line one\nline two\n"),
			wantExit:   0,
			wantStdout: []byte("line one$\nline two$\n"),
		},
		// R4.4: -T displays tab characters as the two-character sequence "^I".
		{
			name:       "flag_T",
			args:       []string{"-T"},
			stdin:      []byte("col1\tcol2\tcol3\n"),
			wantExit:   0,
			wantStdout: []byte("col1^Icol2^Icol3\n"),
		},
		// R4.5: -A is equivalent to -v -E -T combined.
		{
			name:       "flag_A",
			args:       []string{"-A"},
			stdin:      []byte{0x01, '\t', 'h', 'e', 'l', 'l', 'o', '\n'},
			wantExit:   0,
			wantStdout: []byte("^A^Ihello$\n"),
		},
		// R4.6: -e is equivalent to -v -E (show non-printing and show ends).
		{
			name:       "flag_e",
			args:       []string{"-e"},
			stdin:      []byte{0x01, 'h', 'e', 'l', 'l', 'o', '\n'},
			wantExit:   0,
			wantStdout: []byte("^Ahello$\n"),
		},
		// R4.7: -t is equivalent to -v -T (show non-printing and show tabs).
		{
			name:       "flag_t",
			args:       []string{"-t"},
			stdin:      []byte{0x01, '\t', 'h', 'e', 'l', 'l', 'o', '\n'},
			wantExit:   0,
			wantStdout: []byte("^A^Ihello\n"),
		},
		// R4.8: -u is accepted without error and has no effect on output.
		{
			name:       "flag_u_accepted",
			args:       []string{"-u"},
			stdin:      []byte("test\n"),
			wantExit:   0,
			wantStdout: []byte("test\n"),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testutils.DiffTest{
				Args:       tc.args,
				Stdin:      tc.stdin,
				Env:        lcEnv(),
				WantExit:   tc.wantExit,
				WantStdout: tc.wantStdout,
				WantStderr: nil,
			}.Run(t, refCat, goCat)
		})
	}
}

// TestCatDefaultPassthrough verifies that a named file's content is written
// verbatim to stdout with no transformation (prd006-cat R1.1, R1.5).
func TestCatDefaultPassthrough(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	writeFile(t, file1, "hello\nworld\n")

	testutils.DiffTest{
		Args:       []string{file1},
		Env:        lcEnv(),
		WantExit:   0,
		WantStdout: []byte("hello\nworld\n"),
		WantStderr: nil,
	}.Run(t, refCat, goCat)
}

// TestCatMultipleFiles verifies that multiple named files are concatenated in
// argument order with no separator (prd006-cat R1.1, R1.3).
func TestCatMultipleFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")
	writeFile(t, file1, "aaa\n")
	writeFile(t, file2, "bbb\n")

	testutils.DiffTest{
		Args:       []string{file1, file2},
		Env:        lcEnv(),
		WantExit:   0,
		WantStdout: []byte("aaa\nbbb\n"),
		WantStderr: nil,
	}.Run(t, refCat, goCat)
}

// TestCatSqueezeBlanksAcrossFiles verifies that -s applies blank-squeezing
// across file boundaries when multiple files are concatenated (prd006-cat R3.2).
func TestCatSqueezeBlanksAcrossFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file1 := filepath.Join(dir, "file1.txt")
	file2 := filepath.Join(dir, "file2.txt")
	// file1 ends with a blank line; file2 begins with a blank line.
	// With -s, the two adjacent blank lines across the boundary collapse to one.
	writeFile(t, file1, "a\n\n")
	writeFile(t, file2, "\nb\n")

	testutils.DiffTest{
		Args:       []string{"-s", file1, file2},
		Env:        lcEnv(),
		WantExit:   0,
		WantStdout: []byte("a\n\nb\n"),
		WantStderr: nil,
	}.Run(t, refCat, goCat)
}

// TestCatMissingFile verifies that a non-existent file argument causes an error
// message on stderr and exit code 1, while remaining file arguments are still
// processed (prd006-cat R5.1, R5.2).
func TestCatMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	realFile := filepath.Join(dir, "real.txt")
	missingFile := filepath.Join(dir, "nonexistent.txt")
	writeFile(t, realFile, "data\n")

	// R5.2: stderr must contain the path of the missing file. NormStderr
	// normalizes the actual stderr to a sentinel so the comparison is not
	// sensitive to OS-specific error text or the exact error format.
	const sentinelMissing = "nonexistent.txt:not-found"

	testutils.DiffTest{
		Args:       []string{missingFile, realFile},
		Env:        lcEnv(),
		WantExit:   1,
		WantStdout: []byte("data\n"),
		NormStderr: func(actual []byte) []byte {
			if bytes.Contains(actual, []byte(missingFile)) {
				return []byte(sentinelMissing)
			}
			return actual
		},
		WantStderr: []byte(sentinelMissing),
	}.Run(t, refCat, goCat)
}
