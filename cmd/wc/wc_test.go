// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/wc against gwc (GNU coreutils).
// Implements prd005-wc R1.1-R1.4, R2.1-R2.6, R3.1-R3.2, R4.1-R4.3,
// R5.1-R5.2, R6.1-R6.2 test coverage.
// R5.1: all tests set LC_ALL=C via the testutils harness default.
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
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skipf("reference binary gwc not in PATH: %v", err)
	}

	// Create test fixtures in a temp directory.
	tmpDir := t.TempDir()
	writeTestFile(t, tmpDir, "hello.txt", "hello\nworld\n")
	writeTestFile(t, tmpDir, "three-words.txt", "foo bar baz\n")
	writeTestFile(t, tmpDir, "no-trailing-newline.txt", "abc\ndef")
	writeTestFile(t, tmpDir, "empty.txt", "")
	writeTestFile(t, tmpDir, "single-line.txt", "one\n")
	writeTestFile(t, tmpDir, "binary.txt", "\x00\x01\x02\xff\n")
	writeTestFile(t, tmpDir, "multiword.txt", "a b c\nd e f\ng h i\n")

	tests := []testutils.DiffTest{
		// R1.1: default mode (lines, words, bytes) from stdin.
		{
			Name:  "R1.1_default_stdin",
			Stdin: []byte("foo\nbar baz\n"),
		},
		// R1.1: default mode from a file.
		{
			Name:    "R1.1_default_file",
			Args:    []string{filepath.Join(tmpDir, "hello.txt")},
			WorkDir: tmpDir,
		},
		// R1.2: read from stdin when no file arguments.
		{
			Name:  "R1.2_stdin_no_args",
			Stdin: []byte("hello world\n"),
		},
		// R1.2: read from named files in order.
		{
			Name: "R1.2_named_files",
			Args: []string{
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "three-words.txt"),
			},
			WorkDir: tmpDir,
		},
		// R1.3: totals line when multiple files given.
		{
			Name: "R1.3_totals_line",
			Args: []string{
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "three-words.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir: tmpDir,
		},
		// R1.4: "-" means stdin mixed with named files.
		{
			Name: "R1.4_dash_stdin",
			Args: []string{
				filepath.Join(tmpDir, "hello.txt"),
				"-",
			},
			Stdin:   []byte("stdin line\n"),
			WorkDir: tmpDir,
		},

		// R2.1: -l counts newlines.
		{
			Name:  "R2.1_lines_only",
			Args:  []string{"-l"},
			Stdin: []byte("a\nb\nc\n"),
		},
		// R2.1: -l no trailing newline.
		{
			Name:  "R2.1_lines_no_trailing",
			Args:  []string{"-l"},
			Stdin: []byte("a\nb"),
		},
		// R2.2: -w counts words.
		{
			Name:  "R2.2_words_only",
			Args:  []string{"-w"},
			Stdin: []byte("hello world\nfoo bar baz\n"),
		},
		// R2.2: -w with leading/trailing whitespace.
		{
			Name:  "R2.2_words_whitespace",
			Args:  []string{"-w"},
			Stdin: []byte("  hello  world  \n"),
		},
		// R2.3: -c counts bytes.
		{
			Name:  "R2.3_bytes_only",
			Args:  []string{"-c"},
			Stdin: []byte("hello\n"),
		},
		// R2.6: combined flags print in fixed order: lines, words, bytes.
		{
			Name:  "R2.6_combined_lwc",
			Args:  []string{"-l", "-w", "-c"},
			Stdin: []byte("hello world\n"),
		},
		// R2.6: combined flags in reverse order still print l, w, c.
		{
			Name:  "R2.6_combined_cwl",
			Args:  []string{"-c", "-w", "-l"},
			Stdin: []byte("hello world\n"),
		},
		// R2.6: grouped flags.
		{
			Name:  "R2.6_grouped_lwc",
			Args:  []string{"-lwc"},
			Stdin: []byte("hello world\n"),
		},

		// R3.1: multi-file column alignment.
		{
			Name: "R3.1_column_alignment",
			Args: []string{
				filepath.Join(tmpDir, "multiword.txt"),
				filepath.Join(tmpDir, "hello.txt"),
			},
			WorkDir: tmpDir,
		},
		// R3.2: totals line labeled "total".
		{
			Name: "R3.2_total_label",
			Args: []string{
				filepath.Join(tmpDir, "single-line.txt"),
				filepath.Join(tmpDir, "empty.txt"),
			},
			WorkDir: tmpDir,
		},

		// R4.1: stdin via "-".
		{
			Name:  "R4.1_dash_stdin_alone",
			Args:  []string{"-"},
			Stdin: []byte("hello\n"),
		},
		// R4.2: binary input does not corrupt output.
		{
			Name:    "R4.2_binary_input",
			Args:    []string{filepath.Join(tmpDir, "binary.txt")},
			WorkDir: tmpDir,
		},
		// R4.3: empty input produces zero counts.
		{
			Name:  "R4.3_empty_input",
			Stdin: []byte(""),
		},
		// R4.3: empty file produces zero counts.
		{
			Name:    "R4.3_empty_file",
			Args:    []string{filepath.Join(tmpDir, "empty.txt")},
			WorkDir: tmpDir,
		},

		// R5.2: under LC_ALL=C, -m and -c produce identical counts.
		{
			Name:  "R5.2_m_equals_c_under_C",
			Args:  []string{"-m"},
			Stdin: []byte("hello world\n"),
		},

		// R6.1: successful processing exits 0.
		{
			Name:     "R6.1_success_exit_0",
			Args:     []string{filepath.Join(tmpDir, "hello.txt")},
			WorkDir:  tmpDir,
			ExitCode: 0,
		},
		// R6.2: non-existent file exits 1.
		{
			Name:      "R6.2_nonexistent_file",
			Args:      []string{filepath.Join(tmpDir, "does-not-exist.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R6.2: non-existent file mixed with existing — exit 1, still outputs counts.
		{
			Name: "R6.2_nonexistent_mixed",
			Args: []string{
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "does-not-exist.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir:   tmpDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},

		// R2.5: -L max line length.
		{
			Name:  "R2.5_max_line_length",
			Args:  []string{"-L"},
			Stdin: []byte("short\na longer line\nmed\n"),
		},
		// R2.5: -L with tabs (tab-expanded).
		{
			Name:  "R2.5_max_line_tabs",
			Args:  []string{"-L"},
			Stdin: []byte("a\tb\n"),
		},
		// R2.5: -L combined with -l.
		{
			Name:  "R2.5_L_combined_l",
			Args:  []string{"-lL"},
			Stdin: []byte("short\na longer line\n"),
		},

		// R2.1: -l with file argument.
		{
			Name:    "R2.1_lines_file",
			Args:    []string{"-l", filepath.Join(tmpDir, "hello.txt")},
			WorkDir: tmpDir,
		},
		// R2.2: -w with file argument.
		{
			Name:    "R2.2_words_file",
			Args:    []string{"-w", filepath.Join(tmpDir, "three-words.txt")},
			WorkDir: tmpDir,
		},

		// R2.3: -c with file argument.
		{
			Name:    "R2.3_bytes_file",
			Args:    []string{"-c", filepath.Join(tmpDir, "hello.txt")},
			WorkDir: tmpDir,
		},
		// R2.4: -m counts characters (under LC_ALL=C, same as bytes).
		{
			Name:    "R2.4_chars_file",
			Args:    []string{"-m", filepath.Join(tmpDir, "hello.txt")},
			WorkDir: tmpDir,
		},

		// R2.6: -lw combined flags (lines and words only).
		{
			Name:  "R2.6_combined_lw",
			Args:  []string{"-lw"},
			Stdin: []byte("hello world\nfoo bar\n"),
		},
		// R2.6: -lw combined with file argument.
		{
			Name:    "R2.6_combined_lw_file",
			Args:    []string{"-lw", filepath.Join(tmpDir, "multiword.txt")},
			WorkDir: tmpDir,
		},

		// R2.6: default (no flags) with file — prints lines, words, bytes.
		{
			Name:    "R2.6_default_no_flags_file",
			Args:    []string{filepath.Join(tmpDir, "three-words.txt")},
			WorkDir: tmpDir,
		},

		// AC3: two files with totals.
		{
			Name: "AC3_two_files_totals",
			Args: []string{
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "single-line.txt"),
			},
			WorkDir: tmpDir,
		},

		// AC4: multi-file with varying count magnitudes (column width alignment).
		{
			Name: "AC4_varying_magnitudes",
			Args: []string{
				filepath.Join(tmpDir, "empty.txt"),
				filepath.Join(tmpDir, "multiword.txt"),
				filepath.Join(tmpDir, "hello.txt"),
			},
			WorkDir: tmpDir,
		},

		// Multi-file with -l flag only.
		{
			Name: "multi_file_lines_only",
			Args: []string{
				"-l",
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "three-words.txt"),
			},
			WorkDir: tmpDir,
		},
		// Multi-file with -w flag only.
		{
			Name: "multi_file_words_only",
			Args: []string{
				"-w",
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "three-words.txt"),
			},
			WorkDir: tmpDir,
		},

		// R2.5: -L with a file argument (not just stdin).
		{
			Name:    "R2.5_max_line_file",
			Args:    []string{"-L", filepath.Join(tmpDir, "multiword.txt")},
			WorkDir: tmpDir,
		},
		// R2.5: -L with multiple files — total shows max of per-file maxlines.
		{
			Name: "R2.5_max_line_multi_file",
			Args: []string{
				"-L",
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "three-words.txt"),
				filepath.Join(tmpDir, "multiword.txt"),
			},
			WorkDir: tmpDir,
		},
		// R2.5: -L with no trailing newline.
		{
			Name:  "R2.5_max_line_no_trailing_newline",
			Args:  []string{"-L"},
			Stdin: []byte("abc\ndef"),
		},
		// R2.5: -L on empty input.
		{
			Name:  "R2.5_max_line_empty",
			Args:  []string{"-L"},
			Stdin: []byte(""),
		},

		// R2.6: all flags combined (-lwcL) — output order: lines, words, bytes, maxline.
		{
			Name:  "R2.6_all_flags_lwcL",
			Args:  []string{"-lwcL"},
			Stdin: []byte("hello world\nfoo\n"),
		},
		// R2.6: -m and -L combined — output order: chars, maxline.
		{
			Name:  "R2.6_combined_mL",
			Args:  []string{"-mL"},
			Stdin: []byte("hello\tworld\n"),
		},
		// R2.6: -lmL combined — output order: lines, chars, maxline.
		{
			Name:  "R2.6_combined_lmL",
			Args:  []string{"-lmL"},
			Stdin: []byte("short\na longer line here\n"),
		},
		// R2.6: -wcL combined — output order: words, bytes, maxline.
		{
			Name:  "R2.6_combined_wcL",
			Args:  []string{"-wcL"},
			Stdin: []byte("one two three\n"),
		},

		// R3.1: multi-file -L column alignment.
		{
			Name: "R3.1_multi_file_L_alignment",
			Args: []string{
				"-lL",
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "multiword.txt"),
			},
			WorkDir: tmpDir,
		},
		// R3.1: multi-file default mode — column widths match across files and total.
		{
			Name: "R3.1_multi_file_default_alignment",
			Args: []string{
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "three-words.txt"),
				filepath.Join(tmpDir, "no-trailing-newline.txt"),
			},
			WorkDir: tmpDir,
		},

		// R3.2: total line with -L — total maxline is max across files.
		{
			Name: "R3.2_total_max_line",
			Args: []string{
				"-L",
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "three-words.txt"),
			},
			WorkDir: tmpDir,
		},
		// R3.2: total line sums with two files and all default counters.
		{
			Name: "R3.2_total_sums_default",
			Args: []string{
				filepath.Join(tmpDir, "hello.txt"),
				filepath.Join(tmpDir, "multiword.txt"),
			},
			WorkDir: tmpDir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// writeTestFile creates a file with the given content in dir.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", name, err)
	}
}

// normalizeProgramName normalizes error messages for differential comparison.
// GNU wc reports errors as "gwc: file: Error" while our binary uses
// "wc: file: error". This normalizer replaces the program name and lowercases
// the output to eliminate both differences.
func normalizeProgramName(b []byte) []byte {
	b = bytes.ReplaceAll(b, []byte("gwc: "), []byte("wc: "))
	return bytes.ToLower(b)
}
