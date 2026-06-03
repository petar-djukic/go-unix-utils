// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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

// LC_ALL=C ensures locale-independent comparison for -m (R5.1).
var defaultEnv = []string{"LC_ALL=C"}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skip("reference binary gwc not found")
	}
	tests := []testutils.DiffTest{
		{
			Name:  "default_no_flags",
			Args:  nil,
			Stdin: []byte("foo\nbar baz\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_l_only",
			Args:  []string{"-l"},
			Stdin: []byte("one\ntwo\nthree\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_l_no_trailing_newline",
			Args:  []string{"-l"},
			Stdin: []byte("one\ntwo"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_w_only",
			Args:  []string{"-w"},
			Stdin: []byte("  hello   world  \n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_w_multiple_whitespace_types",
			Args:  []string{"-w"},
			Stdin: []byte("a\tb\nc\rd\fe"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_c_only",
			Args:  []string{"-c"},
			Stdin: []byte("hello\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_m_only_ascii",
			Args:  []string{"-m"},
			Stdin: []byte("hello\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_m_lc_all_c_matches_c",
			Args:  []string{"-m"},
			Stdin: []byte("abc\xc0\xc1xyz\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_c_and_m_together_m_wins",
			Args:  []string{"-c", "-m"},
			Stdin: []byte("hello\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_m_then_c_m_wins",
			Args:  []string{"-m", "-c"},
			Stdin: []byte("hello\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lw_combined",
			Args:  []string{"-lw"},
			Stdin: []byte("hello world\ngoodbye\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lwc_combined",
			Args:  []string{"-lwc"},
			Stdin: []byte("test data\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_order_irrelevant_wl",
			Args:  []string{"-w", "-l"},
			Stdin: []byte("one two\nthree\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input",
			Args:  nil,
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input_flag_l",
			Args:  []string{"-l"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input_flag_w",
			Args:  []string{"-w"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input_flag_c",
			Args:  []string{"-c"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "empty_input_flag_m",
			Args:  []string{"-m"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "binary_input",
			Args:  nil,
			Stdin: []byte{0x00, 0x01, 0xff, 0xfe, 0x0a},
			Env:   defaultEnv,
		},
		{
			Name:  "flag_l_single_newline",
			Args:  []string{"-l"},
			Stdin: []byte("\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_w_only_whitespace",
			Args:  []string{"-w"},
			Stdin: []byte("   \t\n  \n"),
			Env:   defaultEnv,
		},
		{
			Name:  "all_flags_combined",
			Args:  []string{"-l", "-w", "-c"},
			Stdin: []byte("hello world\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lm_combined",
			Args:  []string{"-l", "-m"},
			Stdin: []byte("abc\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_wm_combined",
			Args:  []string{"-w", "-m"},
			Stdin: []byte("abc def\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lwm_combined",
			Args:  []string{"-l", "-w", "-m"},
			Stdin: []byte("abc def\nghi\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_L_only",
			Args:  []string{"-L"},
			Stdin: []byte("short\na longer line here\nmed\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_L_with_tabs",
			Args:  []string{"-L"},
			Stdin: []byte("a\tb\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_L_multiple_lines",
			Args:  []string{"-L"},
			Stdin: []byte("ab\n1234567890\nxyz\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_L_no_trailing_newline",
			Args:  []string{"-L"},
			Stdin: []byte("hello\nworld"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_L_empty",
			Args:  []string{"-L"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lL_combined",
			Args:  []string{"-l", "-L"},
			Stdin: []byte("hello world\ngoodbye\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_wL_combined",
			Args:  []string{"-w", "-L"},
			Stdin: []byte("hello world\ngoodbye\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_cL_combined",
			Args:  []string{"-c", "-L"},
			Stdin: []byte("hello world\ngoodbye\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lwcL_combined",
			Args:  []string{"-l", "-w", "-c", "-L"},
			Stdin: []byte("test data\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flags_lwmL_combined",
			Args:  []string{"-l", "-w", "-m", "-L"},
			Stdin: []byte("test data\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "flag_order_Llw",
			Args:  []string{"-L", "-l", "-w"},
			Stdin: []byte("one two\nthree\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "r1_1_default_counts_order",
			Args:  nil,
			Stdin: []byte("one two three\nfour five\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "r1_1_default_no_trailing_newline",
			Args:  nil,
			Stdin: []byte("hello world"),
			Env:   defaultEnv,
		},
		{
			Name:  "r1_1_default_single_newline",
			Args:  nil,
			Stdin: []byte("\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "r1_2_stdin_dash_arg",
			Args:  []string{"-"},
			Stdin: []byte("hello\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "r1_2_stdin_dash_default_flags",
			Args:  []string{"-"},
			Stdin: []byte("one two\nthree\n"),
			Env:   defaultEnv,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func writeFixture(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiffFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skip("reference binary gwc not found")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "short.txt", "hi\n")
	writeFixture(t, dir, "longer.txt", "hello world\nthis is a longer file with more content\n")
	writeFixture(t, dir, "empty.txt", "")
	writeFixture(t, dir, "binary.dat", "\x00\x01\xff\xfe\x0a")
	writeFixture(t, dir, "filelist_two.txt", "short.txt\x00longer.txt\x00")
	writeFixture(t, dir, "filelist_one.txt", "short.txt\x00")
	writeFixture(t, dir, "filelist_three.txt", "short.txt\x00longer.txt\x00empty.txt\x00")

	tests := []testutils.DiffTest{
		{
			Name:    "multi_file_default",
			Args:    []string{"short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "multi_file_flag_l",
			Args:    []string{"-l", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "multi_file_flag_w",
			Args:    []string{"-w", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "multi_file_flag_c",
			Args:    []string{"-c", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "multi_file_flag_L",
			Args:    []string{"-L", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "multi_file_flags_lwc",
			Args:    []string{"-l", "-w", "-c", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "multi_file_flags_lwcL",
			Args:    []string{"-l", "-w", "-c", "-L", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "multi_file_three_files",
			Args:    []string{"short.txt", "longer.txt", "empty.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "single_file_no_total",
			Args:    []string{"short.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r1_3_single_file_with_name",
			Args:    []string{"longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r1_4_two_files_total_line",
			Args:    []string{"short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r1_4_three_files_total_line",
			Args:    []string{"empty.txt", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r1_4_total_with_empty_file",
			Args:    []string{"empty.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_always_single",
			Args:    []string{"--total=always", "short.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_always_multi",
			Args:    []string{"--total=always", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_only_single",
			Args:    []string{"--total=only", "short.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_only_multi",
			Args:    []string{"--total=only", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_never_single",
			Args:    []string{"--total=never", "short.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_never_multi",
			Args:    []string{"--total=never", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_auto_single",
			Args:    []string{"--total=auto", "short.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_auto_multi",
			Args:    []string{"--total=auto", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_only_flags_lwc",
			Args:    []string{"--total=only", "-l", "-w", "-c", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_never_flags_lwc",
			Args:    []string{"--total=never", "-l", "-w", "-c", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r3_3_total_always_three_files",
			Args:    []string{"--total=always", "empty.txt", "short.txt", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_3_empty_file_alone",
			Args:    []string{"empty.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_3_empty_file_flag_l",
			Args:    []string{"-l", "empty.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_3_empty_file_flag_c",
			Args:    []string{"-c", "empty.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_2_binary_file",
			Args:    []string{"binary.dat"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_2_binary_file_flag_c",
			Args:    []string{"-c", "binary.dat"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_2_binary_file_flag_m",
			Args:    []string{"-m", "binary.dat"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_2_binary_file_flag_L",
			Args:    []string{"-L", "binary.dat"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_4_files0_from_single",
			Args:    []string{"--files0-from=filelist_one.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_4_files0_from_multi",
			Args:    []string{"--files0-from=filelist_two.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_4_files0_from_three",
			Args:    []string{"--files0-from=filelist_three.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_4_files0_from_flag_l",
			Args:    []string{"--files0-from=filelist_two.txt", "-l"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_4_files0_from_flag_w",
			Args:    []string{"--files0-from=filelist_two.txt", "-w"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_4_files0_from_flags_lwc",
			Args:    []string{"--files0-from=filelist_two.txt", "-l", "-w", "-c"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r5_2_m_equals_c_ascii_file",
			Args:    []string{"-m", "short.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r5_2_c_ascii_file",
			Args:    []string{"-c", "short.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r5_2_m_equals_c_longer_file",
			Args:    []string{"-m", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r5_2_c_longer_file",
			Args:    []string{"-c", "longer.txt"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r5_2_m_equals_c_binary_file",
			Args:    []string{"-m", "binary.dat"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r5_2_c_binary_file",
			Args:    []string{"-c", "binary.dat"},
			Env:     defaultEnv,
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFiles0Stdin(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skip("reference binary gwc not found")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "alpha.txt", "hello\n")
	writeFixture(t, dir, "beta.txt", "world foo\nbar\n")

	tests := []testutils.DiffTest{
		{
			Name:    "r4_4_files0_from_stdin_single",
			Args:    []string{"--files0-from=-"},
			Stdin:   []byte("alpha.txt\x00"),
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_4_files0_from_stdin_multi",
			Args:    []string{"--files0-from=-"},
			Stdin:   []byte("alpha.txt\x00beta.txt\x00"),
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_4_files0_from_stdin_flag_l",
			Args:    []string{"--files0-from=-", "-l"},
			Stdin:   []byte("alpha.txt\x00beta.txt\x00"),
			Env:     defaultEnv,
			WorkDir: dir,
		},
		{
			Name:    "r4_4_files0_from_stdin_flags_lwc",
			Args:    []string{"--files0-from=-", "-l", "-w", "-c"},
			Stdin:   []byte("alpha.txt\x00beta.txt\x00"),
			Env:     defaultEnv,
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffR52(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skip("reference binary gwc not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:  "r5_2_m_stdin_ascii",
			Args:  []string{"-m"},
			Stdin: []byte("hello world\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "r5_2_c_stdin_ascii",
			Args:  []string{"-c"},
			Stdin: []byte("hello world\n"),
			Env:   defaultEnv,
		},
		{
			Name:  "r5_2_m_stdin_binary",
			Args:  []string{"-m"},
			Stdin: []byte{0xc0, 0xc1, 0x61, 0x62, 0x0a},
			Env:   defaultEnv,
		},
		{
			Name:  "r5_2_c_stdin_binary",
			Args:  []string{"-c"},
			Stdin: []byte{0xc0, 0xc1, 0x61, 0x62, 0x0a},
			Env:   defaultEnv,
		},
		{
			Name:  "r5_2_m_stdin_empty",
			Args:  []string{"-m"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
		{
			Name:  "r5_2_c_stdin_empty",
			Args:  []string{"-c"},
			Stdin: []byte(""),
			Env:   defaultEnv,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffR62(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gwc")
	if err != nil {
		t.Skip("reference binary gwc not found")
	}

	dir := t.TempDir()
	writeFixture(t, dir, "valid.txt", "hello world\nfoo bar\n")

	binaryNameRe := regexp.MustCompile(`(?:/\S+/)?g?wc`)
	normalizeBinaryName := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("wc"))
	})
	normalizeErrCase := testutils.NormalizeFunc(func(b []byte) []byte {
		return bytes.ToLower(b)
	})

	tests := []testutils.DiffTest{
		{
			Name:      "r6_2_nonexistent_file_only",
			Args:      []string{"nonexistent.txt"},
			Env:       defaultEnv,
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrCase},
		},
		{
			Name:      "r6_2_nonexistent_then_valid",
			Args:      []string{"nonexistent.txt", "valid.txt"},
			Env:       defaultEnv,
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrCase},
		},
		{
			Name:      "r6_2_valid_then_nonexistent",
			Args:      []string{"valid.txt", "nonexistent.txt"},
			Env:       defaultEnv,
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrCase},
		},
		{
			Name:      "r6_2_nonexistent_between_valid",
			Args:      []string{"valid.txt", "nonexistent.txt", "valid.txt"},
			Env:       defaultEnv,
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrCase},
		},
		{
			Name:      "r6_2_nonexistent_flag_l",
			Args:      []string{"-l", "nonexistent.txt", "valid.txt"},
			Env:       defaultEnv,
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrCase},
		},
		{
			Name:      "r6_2_nonexistent_flag_w",
			Args:      []string{"-w", "nonexistent.txt", "valid.txt"},
			Env:       defaultEnv,
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrCase},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}
