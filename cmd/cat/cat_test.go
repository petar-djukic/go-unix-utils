// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/cat against gcat (GNU coreutils).
// Implements prd006-cat R1.5, R2.1-R2.4, R3.1-R3.3, R4.1-R4.9, R5.1-R5.4 test coverage.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcat")
	if err != nil {
		t.Skipf("reference binary gcat not in PATH: %v", err)
	}

	// Create test fixtures in a temp directory.
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "hello.txt", "hello\nworld\n")
	writeTestFile(t, tmpDir, "no-trailing-newline.txt", "abc\ndef")
	writeTestFile(t, tmpDir, "blanks.txt", "a\n\n\n\nb\n")
	writeTestFile(t, tmpDir, "single-line.txt", "one\n")
	writeTestFile(t, tmpDir, "empty.txt", "")
	writeTestFile(t, tmpDir, "trailing-blanks.txt", "x\n\n\n")
	writeTestFile(t, tmpDir, "leading-blanks.txt", "\n\ny\n")

	tests := []testutils.DiffTest{
		// R1.5: no newlines added or removed — no trailing newline preserved.
		{
			Name:    "R1.5_no_trailing_newline_preserved",
			Args:    []string{filepath.Join(tmpDir, "no-trailing-newline.txt")},
			WorkDir: tmpDir,
		},
		// R1.5: empty file produces no output.
		{
			Name:    "R1.5_empty_file",
			Args:    []string{filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
		},
		// R1.5: no newlines added via stdin.
		{
			Name:  "R1.5_stdin_no_trailing_newline",
			Stdin: []byte("abc\ndef"),
		},

		// R2.1: -n numbers all lines.
		{
			Name:  "R2.1_number_all_lines",
			Args:  []string{"-n"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.1: -n numbers blank lines too.
		{
			Name:  "R2.1_number_blank_lines",
			Args:  []string{"-n"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R2.1: -n with no trailing newline.
		{
			Name:  "R2.1_number_no_trailing_newline",
			Args:  []string{"-n"},
			Stdin: []byte("a\nb"),
		},
		// R2.1: -n numbering continues across files.
		{
			Name: "R2.1_number_across_files",
			Args: []string{
				"-n",
				filepath.Join(tmpDir, "single-line.txt"),
				filepath.Join(tmpDir, "hello.txt"),
			},
			WorkDir: tmpDir,
		},
		// R2.1: -n with single line.
		{
			Name:  "R2.1_single_line",
			Args:  []string{"-n"},
			Stdin: []byte("only\n"),
		},
		// R2.1: -n with empty stdin.
		{
			Name:  "R2.1_empty_stdin",
			Args:  []string{"-n"},
			Stdin: []byte(""),
		},

		// R2.2: -b numbers only non-blank lines.
		{
			Name:  "R2.2_number_nonblank",
			Args:  []string{"-b"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R2.2: -b blank lines have no prefix.
		{
			Name:  "R2.2_blank_no_prefix",
			Args:  []string{"-b"},
			Stdin: []byte("\n\na\n\nb\n\n"),
		},
		// R2.2: -b with spaces-only line (not blank per R2.4).
		{
			Name:  "R2.2_spaces_not_blank",
			Args:  []string{"-b"},
			Stdin: []byte("a\n  \n\nb\n"),
		},

		// R2.3: -b overrides -n when both given.
		{
			Name:  "R2.3_b_overrides_n",
			Args:  []string{"-n", "-b"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R2.3: -nb combined flag.
		{
			Name:  "R2.3_nb_combined",
			Args:  []string{"-nb"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R2.3: -bn order (b first, n second).
		{
			Name:  "R2.3_bn_combined",
			Args:  []string{"-bn"},
			Stdin: []byte("a\n\nb\n"),
		},

		// R2.4: tabs-only line is not blank (contains non-newline bytes).
		{
			Name:  "R2.4_tab_not_blank",
			Args:  []string{"-b"},
			Stdin: []byte("a\n\t\n\nb\n"),
		},

		// R3.1: -s suppresses repeated blank lines.
		{
			Name:  "R3.1_squeeze_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.1: -s single blank line passes through.
		{
			Name:  "R3.1_single_blank_passes",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\nb\n"),
		},
		// R3.1: -s leading blank lines squeezed.
		{
			Name:  "R3.1_leading_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("\n\n\na\n"),
		},
		// R3.1: -s trailing blank lines squeezed.
		{
			Name:  "R3.1_trailing_blanks",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n\n"),
		},
		// R3.1: -s spaces-only line is not blank.
		{
			Name:  "R3.1_spaces_not_blank",
			Args:  []string{"-s"},
			Stdin: []byte("a\n\n \n\nb\n"),
		},

		// R3.2: -s applies across file boundaries.
		{
			Name: "R3.2_squeeze_across_files",
			Args: []string{
				"-s",
				filepath.Join(tmpDir, "trailing-blanks.txt"),
				filepath.Join(tmpDir, "leading-blanks.txt"),
			},
			WorkDir: tmpDir,
		},

		// R3.3: -s combined with -n — suppressed lines don't consume numbers.
		{
			Name:  "R3.3_squeeze_with_number",
			Args:  []string{"-sn"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},
		// R3.3: -s combined with -b — squeeze before numbering.
		{
			Name:  "R3.3_squeeze_with_number_nonblank",
			Args:  []string{"-sb"},
			Stdin: []byte("a\n\n\n\nb\n"),
		},

		// R4.1: -v displays control characters with caret notation.
		{
			Name:  "R4.1_v_control_chars",
			Args:  []string{"-v"},
			Stdin: []byte{0x01, 0x02, 0x03, 0x1B, '\n'},
		},
		// R4.1: -v displays DEL as ^?.
		{
			Name:  "R4.1_v_del",
			Args:  []string{"-v"},
			Stdin: []byte{0x7F, '\n'},
		},
		// R4.1: -v displays high bytes with M- prefix.
		{
			Name:  "R4.1_v_high_bytes",
			Args:  []string{"-v"},
			Stdin: []byte{0x80, 0x9F, 0xA0, 0xFE, 0xFF, '\n'},
		},
		// R4.1: -v mixed printable and non-printing.
		{
			Name:  "R4.1_v_mixed",
			Args:  []string{"-v"},
			Stdin: []byte("hello\x01world\x7f\n"),
		},

		// R4.2: -v does not alter tab or newline.
		{
			Name:  "R4.2_v_preserves_tab_newline",
			Args:  []string{"-v"},
			Stdin: []byte("a\tb\n"),
		},

		// R4.3: -E appends "$" before newlines.
		{
			Name:  "R4.3_E_show_ends",
			Args:  []string{"-E"},
			Stdin: []byte("hello\nworld\n"),
		},
		// R4.3: -E with blank lines.
		{
			Name:  "R4.3_E_blank_lines",
			Args:  []string{"-E"},
			Stdin: []byte("a\n\n\nb\n"),
		},
		// R4.3: -E no trailing newline.
		{
			Name:  "R4.3_E_no_trailing_newline",
			Args:  []string{"-E"},
			Stdin: []byte("abc"),
		},

		// R4.4: -T displays tabs as ^I.
		{
			Name:  "R4.4_T_show_tabs",
			Args:  []string{"-T"},
			Stdin: []byte("a\tb\tc\n"),
		},
		// R4.4: -T with multiple tabs.
		{
			Name:  "R4.4_T_multiple_tabs",
			Args:  []string{"-T"},
			Stdin: []byte("\t\thello\t\n"),
		},

		// R4.5: -A combines -v -E -T.
		{
			Name:  "R4.5_A_all",
			Args:  []string{"-A"},
			Stdin: []byte("a\tb\x01\n"),
		},

		// R4.6: -e combines -v -E.
		{
			Name:  "R4.6_e_nonprinting_ends",
			Args:  []string{"-e"},
			Stdin: []byte("a\tb\x01\n"),
		},

		// R4.7: -t combines -v -T.
		{
			Name:  "R4.7_t_nonprinting_tabs",
			Args:  []string{"-t"},
			Stdin: []byte("a\tb\x01\n"),
		},

		// R4.8: -u is accepted but has no effect.
		{
			Name:  "R4.8_u_accepted_no_effect",
			Args:  []string{"-u"},
			Stdin: []byte("hello\nworld\n"),
		},
		// R4.8: -u combined with other flags.
		{
			Name:  "R4.8_u_combined_with_n",
			Args:  []string{"-un"},
			Stdin: []byte("a\nb\n"),
		},

		// R4.9: transformation order with -n and -A combined.
		{
			Name:  "R4.9_nA_combined",
			Args:  []string{"-nA"},
			Stdin: []byte("a\tb\x01\n\nc\n"),
		},
		// R4.9: -sbA combined (squeeze + nonprinting + ends + tabs + number nonblank).
		{
			Name:  "R4.9_sbA_combined",
			Args:  []string{"-sbA"},
			Stdin: []byte("a\n\n\n\nb\t\x01\n"),
		},
		// R4.9: all transformation flags combined with squeeze and numbering.
		{
			Name:  "R4.9_all_flags_snvET",
			Args:  []string{"-snvET"},
			Stdin: []byte("a\x01\tb\n\n\n\nc\x7f\n"),
		},

		// R5.1: successful processing exits 0.
		{
			Name:     "R5.1_success_exit_0",
			Args:     []string{filepath.Join(tmpDir, "hello.txt")},
			WorkDir:  tmpDir,
			ExitCode: 0,
		},

		// R5.2: non-existent file exits 1.
		{
			Name:      "R5.2_nonexistent_file",
			Args:      []string{filepath.Join(tmpDir, "does-not-exist.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R5.2: non-existent file mixed with existing — exit 1, still outputs
		// the content of existing files.
		{
			Name: "R5.2_nonexistent_mixed",
			Args: []string{
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "does-not-exist.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestSIGPIPEExit verifies that cat exits 0 when stdout is closed by a
// downstream consumer (SIGPIPE), per R5.3 and R5.4. R5.4 requires exit 0
// on SIGPIPE; R5.3 requires detection of stdout write errors (SIGPIPE is
// the primary stdout write error on Unix).
//
// Note: This is not a differential test against gcat because GNU cat (C)
// is killed by SIGPIPE (exit 141 = 128+13) which is the default C signal
// disposition. Go's runtime changes the default SIGPIPE disposition, so
// the Go binary must install an explicit handler to exit 0. Both behaviors
// are correct for their respective runtimes — pipelines work correctly
// because shells special-case signal-killed processes.
func TestSIGPIPEExit(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")

	bashBin, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not found in PATH")
	}

	// R5.4: cat must exit 0 when downstream consumer closes stdout.
	// Feed cat input from /dev/zero; pipe to head -c 1 which reads one byte
	// then closes, triggering SIGPIPE in cat. Use bash PIPESTATUS to capture
	// cat's exit code specifically.
	t.Run("R5.4_sigpipe_exit_0", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(bashBin, "-c",
			fmt.Sprintf("'%s' /dev/zero | head -c 1 > /dev/null; exit ${PIPESTATUS[0]}", goBin))
		if err := cmd.Run(); err != nil {
			t.Errorf("R5.4: expected cat to exit 0 on SIGPIPE, got: %v", err)
		}
	})

	// R5.4: SIGPIPE with transformation flags active — cat must still exit 0
	// when line-by-line processing is engaged and downstream closes.
	// Use yes to produce newlines so line-by-line processing generates output.
	t.Run("R5.4_sigpipe_with_flags", func(t *testing.T) {
		t.Parallel()
		cmd := exec.Command(bashBin, "-c",
			fmt.Sprintf("yes | '%s' -n | head -c 1 > /dev/null; exit ${PIPESTATUS[1]}", goBin))
		if err := cmd.Run(); err != nil {
			t.Errorf("R5.4: expected cat -n to exit 0 on SIGPIPE, got: %v", err)
		}
	})
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", name, err)
	}
}

// normalizeProgramName normalizes error messages for differential comparison.
// R5.2: GNU cat reports errors as "gcat: file: Error" while our binary uses
// "cat: file: error". This normalizer replaces the program name and lowercases
// the output to eliminate both differences.
func normalizeProgramName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gcat: "), []byte("cat: "))
	return bytes.ToLower(b)
}
