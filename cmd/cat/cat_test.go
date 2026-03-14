// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat.
//
// Implements: prd006-cat R4.9, R5.1-R5.4
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const binGcat = "gcat"

// catErrRe matches a cat or gcat error line: normalizes both the program name
// ("cat" vs "gcat") and the "open " prefix that Go's os.Open errors include
// but GNU cat omits.
//
// Matches:
//
//	cat: open /path/to/file: no such file or directory
//	gcat: /path/to/file: No such file or directory
var catErrRe = regexp.MustCompile(`(?m)^g?cat: (?:open )?(.+?): .+$`)

// normalizeCatErrors replaces cat/gcat error lines with a canonical form so
// that program-name and error-message-format differences do not cause false
// test failures. Applied to both stdout and stderr; non-error lines pass through
// unchanged because they do not match the prefix "cat:" or "gcat:".
func normalizeCatErrors(b []byte) []byte {
	return catErrRe.ReplaceAll(b, []byte("PROG: $1: ERROR"))
}

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(binGcat)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", binGcat, err)
	}

	// Set up temp files shared across file-based test cases.
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.txt")
	fileB := filepath.Join(dir, "b.txt")
	missing := filepath.Join(dir, "nonexistent.txt")

	if err := os.WriteFile(fileA, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatalf("writing a.txt: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("foo\nbar\n"), 0o644); err != nil {
		t.Fatalf("writing b.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.9: -n with -v — line number prefix appears before display-transformed content.
		{
			Name:  "r4.9_n_v_control_chars",
			Args:  []string{"-n", "-v"},
			Stdin: []byte("\x01hello\n\x1bworld\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -b with -v — blank lines skip numbering; non-blank lines get number + display.
		{
			Name:  "r4.9_b_v_blanks_and_control",
			Args:  []string{"-b", "-v"},
			Stdin: []byte("line1\n\n\x01line2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -s with -n — squeeze applied before numbering; suppressed blanks consume no line numbers.
		{
			Name:  "r4.9_s_n_squeeze_before_number",
			Args:  []string{"-s", "-n"},
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -s with -b — squeeze applied before non-blank numbering.
		{
			Name:  "r4.9_s_b_squeeze_before_nonblank_number",
			Args:  []string{"-s", "-b"},
			Stdin: []byte("a\n\n\n\nb\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -n with -E — line number before end marker.
		{
			Name:  "r4.9_n_E_number_before_end",
			Args:  []string{"-n", "-E"},
			Stdin: []byte("alpha\nbeta\ngamma\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -n with -v -E — number, display, and end marker all combined.
		{
			Name:  "r4.9_n_v_E_combined",
			Args:  []string{"-n", "-v", "-E"},
			Stdin: []byte("\x01hello\n\n\x09world\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -n with -T — tab displayed as ^I, with line numbers.
		{
			Name:  "r4.9_n_T_tabs_numbered",
			Args:  []string{"-n", "-T"},
			Stdin: []byte("col1\tcol2\n\tcol3\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -A with -n — all transforms (-v -E -T) plus line numbering.
		{
			Name:  "r4.9_A_n_all_transforms_numbered",
			Args:  []string{"-A", "-n"},
			Stdin: []byte("a\x01b\tc\n\n\x7f\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -s with -v — squeeze blanks, then display non-printing on remaining lines.
		{
			Name:  "r4.9_s_v_squeeze_then_display",
			Args:  []string{"-s", "-v"},
			Stdin: []byte("a\n\n\n\x01b\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -b with -s and -v — three flags combined.
		{
			Name:  "r4.9_b_s_v_three_flags",
			Args:  []string{"-b", "-s", "-v"},
			Stdin: []byte("\x01line1\n\n\n\x02line2\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R4.9: -s with -E — end markers on squeezed output.
		{
			Name:  "r4.9_s_E_squeeze_with_ends",
			Args:  []string{"-s", "-E"},
			Stdin: []byte("x\n\n\n\ny\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R5.1: stdin → exit 0.
		{
			Name:     "r5.1_exit0_stdin",
			Stdin:    []byte("hello world\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R5.1: named file → exit 0.
		{
			Name:     "r5.1_exit0_named_file",
			Args:     []string{fileA},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R5.1: multiple named files → exit 0.
		{
			Name:     "r5.1_exit0_multiple_files",
			Args:     []string{fileA, fileB},
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R5.2: missing file only → exit 1, error written to stderr.
		{
			Name:      "r5.2_missing_file_exit1",
			Args:      []string{missing},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCatErrors},
		},
		// R5.2: missing file followed by existing file → exit 1, continues processing
		// (existing file content appears on stdout).
		{
			Name:      "r5.2_missing_then_existing_continues",
			Args:      []string{missing, fileA},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCatErrors},
		},
		// R5.2: existing file, missing file, existing file → exit 1, both flanking
		// files contribute to stdout.
		{
			Name:      "r5.2_existing_missing_existing",
			Args:      []string{fileA, missing, fileB},
			Env:       []string{"LC_ALL=C"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeCatErrors},
		},
		// R5.3: large stdin write succeeds → exit 0 (no spurious write errors).
		{
			Name:     "r5.3_large_stdin_exits0",
			Stdin:    bytes.Repeat([]byte("x\n"), 100000),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R5.3: large stdin with line-number transformation succeeds → exit 0.
		{
			Name:     "r5.3_large_stdin_with_n_exits0",
			Args:     []string{"-n"},
			Stdin:    bytes.Repeat([]byte("data line\n"), 10000),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R5.3: large stdin with all transforms succeeds → exit 0.
		{
			Name:     "r5.3_large_stdin_with_A_exits0",
			Args:     []string{"-A"},
			Stdin:    bytes.Repeat([]byte("line\t\x01\n"), 10000),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R5.4: SIGPIPE handler installed at startup using signal.Notify (not signal.Ignore).
		// Binary data containing null bytes and control characters passes through unchanged
		// and exits 0. Verifies the dedicated SIGPIPE goroutine does not interfere with
		// binary passthrough or cause spurious non-zero exits.
		{
			Name:     "r5.4_sigpipe_handler_binary_null_bytes_exit0",
			Stdin:    []byte("\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0d\x0e\x0f\n"),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
		// R5.4: SIGPIPE handler does not interfere with high-volume binary output.
		// Uses bytes in the 0x80–0xFF range to exercise high-byte passthrough with exit 0.
		{
			Name:     "r5.4_sigpipe_handler_high_byte_passthrough",
			Stdin:    bytes.Repeat([]byte("\x80\x9f\xa0\xfe\xff\n"), 10000),
			Env:      []string{"LC_ALL=C"},
			ExitCode: 0,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
