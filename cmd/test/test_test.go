// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/test against gtest reference binary.
// Implements: srd104-test R1.1, R1.2, R2.1, R2.2, R3.1, R3.2, R4.1, R4.2.
package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer strips the program name prefix from error messages so
// "gtest: ..." and "test: ..." compare equal.
var stderrNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`(?m)^(?:gtest|test): `)
	return re.ReplaceAll(b, []byte("test: "))
}

// testFixtures holds paths to temporary test fixtures for file operator tests.
type testFixtures struct {
	regFile, emptyFile, subDir, symlink string
	brokenLink, fifo, execFile          string
	olderFile, newerFile, hardLink      string
	nonExist                            string
}

func setupFixtures(t *testing.T) testFixtures {
	t.Helper()
	dir := t.TempDir()
	f := testFixtures{
		regFile:    filepath.Join(dir, "regular.txt"),
		emptyFile:  filepath.Join(dir, "empty.txt"),
		subDir:     filepath.Join(dir, "subdir"),
		symlink:    filepath.Join(dir, "link"),
		brokenLink: filepath.Join(dir, "broken_link"),
		fifo:       filepath.Join(dir, "fifo"),
		execFile:   filepath.Join(dir, "exec.sh"),
		olderFile:  filepath.Join(dir, "older.txt"),
		newerFile:  filepath.Join(dir, "newer.txt"),
		hardLink:   filepath.Join(dir, "hardlink.txt"),
		nonExist:   filepath.Join(dir, "nonexistent"),
	}
	writeFixture(t, f.regFile, "content\n", 0o644)
	writeFixture(t, f.emptyFile, "", 0o644)
	writeFixture(t, f.execFile, "#!/bin/sh\n", 0o755)
	writeFixture(t, f.olderFile, "old", 0o644)
	old := time.Now().Add(-10 * time.Second)
	failOnErr(t, os.Chtimes(f.olderFile, old, old))
	writeFixture(t, f.newerFile, "new", 0o644)
	failOnErr(t, os.Mkdir(f.subDir, 0o755))
	failOnErr(t, os.Symlink(f.regFile, f.symlink))
	noTarget := filepath.Join(dir, "no_target")
	failOnErr(t, os.Symlink(noTarget, f.brokenLink))
	failOnErr(t, syscall.Mkfifo(f.fifo, 0o644))
	failOnErr(t, os.Link(f.regFile, f.hardLink))
	return f
}

func writeFixture(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	failOnErr(t, os.WriteFile(path, []byte(content), perm))
}

func failOnErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gtest")
	if err != nil {
		t.Skipf("reference binary gtest not in PATH: %v", err)
	}
	f := setupFixtures(t)
	var tests []testutils.DiffTest
	tests = append(tests, stringTests()...)
	tests = append(tests, intTests()...)
	tests = append(tests, logicalTests()...)
	tests = append(tests, fileExistTypeTests(f)...)
	tests = append(tests, fileAttrTests(f)...)
	tests = append(tests, fileCompareTests(f)...)
	tests = append(tests, errorTests()...)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// stringTests returns differential tests for string operators (R2.1).
func stringTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "zero_args_false", Args: []string{}, ExitCode: 1},
		{Name: "single_nonempty_string", Args: []string{"hello"}, ExitCode: 0},
		{Name: "single_empty_string", Args: []string{""}, ExitCode: 1},
		{Name: "z_empty", Args: []string{"-z", ""}, ExitCode: 0},
		{Name: "z_nonempty", Args: []string{"-z", "abc"}, ExitCode: 1},
		{Name: "n_nonempty", Args: []string{"-n", "abc"}, ExitCode: 0},
		{Name: "n_empty", Args: []string{"-n", ""}, ExitCode: 1},
		{Name: "string_equal", Args: []string{"abc", "=", "abc"}, ExitCode: 0},
		{Name: "string_not_equal", Args: []string{"abc", "=", "def"}, ExitCode: 1},
		{Name: "string_neq", Args: []string{"abc", "!=", "def"}, ExitCode: 0},
		{Name: "string_neq_same", Args: []string{"abc", "!=", "abc"}, ExitCode: 1},
	}
}

// intTests returns differential tests for integer comparison operators (R2.2).
func intTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "int_eq_true", Args: []string{"1", "-eq", "1"}, ExitCode: 0},
		{Name: "int_eq_false", Args: []string{"1", "-eq", "2"}, ExitCode: 1},
		{Name: "int_ne_true", Args: []string{"1", "-ne", "2"}, ExitCode: 0},
		{Name: "int_ne_false", Args: []string{"1", "-ne", "1"}, ExitCode: 1},
		{Name: "int_lt_true", Args: []string{"1", "-lt", "2"}, ExitCode: 0},
		{Name: "int_lt_false", Args: []string{"2", "-lt", "1"}, ExitCode: 1},
		{Name: "int_le_true", Args: []string{"1", "-le", "1"}, ExitCode: 0},
		{Name: "int_le_false", Args: []string{"2", "-le", "1"}, ExitCode: 1},
		{Name: "int_gt_true", Args: []string{"2", "-gt", "1"}, ExitCode: 0},
		{Name: "int_gt_false", Args: []string{"1", "-gt", "2"}, ExitCode: 1},
		{Name: "int_ge_true", Args: []string{"2", "-ge", "2"}, ExitCode: 0},
		{Name: "int_ge_false", Args: []string{"1", "-ge", "2"}, ExitCode: 1},
		{Name: "int_negative_lt", Args: []string{"-1", "-lt", "0"}, ExitCode: 0},
		{
			Name: "int_invalid_operand", Args: []string{"abc", "-eq", "1"},
			ExitCode: 2, Normalize: []testutils.NormalizeFunc{stderrNormalizer},
		},
	}
}

// logicalTests returns differential tests for logical operators (R3.1).
func logicalTests() []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "negation_true", Args: []string{"!", ""}, ExitCode: 0},
		{Name: "negation_false", Args: []string{"!", "hello"}, ExitCode: 1},
		{Name: "and_both_true", Args: []string{"abc", "-a", "def"}, ExitCode: 0},
		{Name: "and_one_false", Args: []string{"abc", "-a", ""}, ExitCode: 1},
		{Name: "or_one_true", Args: []string{"", "-o", "abc"}, ExitCode: 0},
		{Name: "or_both_false", Args: []string{"", "-o", ""}, ExitCode: 1},
		{Name: "parens_group", Args: []string{"(", "abc", ")"}, ExitCode: 0},
		{Name: "not_d_nonexistent", Args: []string{"!", "-d", "/nonexistent_path_xyz"}, ExitCode: 0},
	}
}

// fileExistTypeTests returns differential tests for -e, -f, -d operators (R3.1).
func fileExistTypeTests(f testFixtures) []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "e_regular", Args: []string{"-e", f.regFile}, ExitCode: 0},
		{Name: "e_dir", Args: []string{"-e", f.subDir}, ExitCode: 0},
		{Name: "e_symlink", Args: []string{"-e", f.symlink}, ExitCode: 0},
		{Name: "e_broken_link", Args: []string{"-e", f.brokenLink}, ExitCode: 1},
		{Name: "e_nonexistent", Args: []string{"-e", f.nonExist}, ExitCode: 1},
		{Name: "f_regular", Args: []string{"-f", f.regFile}, ExitCode: 0},
		{Name: "f_dir", Args: []string{"-f", f.subDir}, ExitCode: 1},
		{Name: "f_symlink_to_file", Args: []string{"-f", f.symlink}, ExitCode: 0},
		{Name: "f_nonexistent", Args: []string{"-f", f.nonExist}, ExitCode: 1},
		{Name: "d_dir", Args: []string{"-d", f.subDir}, ExitCode: 0},
		{Name: "d_regular", Args: []string{"-d", f.regFile}, ExitCode: 1},
		{Name: "d_nonexistent", Args: []string{"-d", f.nonExist}, ExitCode: 1},
	}
}

// fileAttrTests returns differential tests for -s, -r, -w, -x, -L, -h, -p (R3.1).
func fileAttrTests(f testFixtures) []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "s_nonempty", Args: []string{"-s", f.regFile}, ExitCode: 0},
		{Name: "s_empty", Args: []string{"-s", f.emptyFile}, ExitCode: 1},
		{Name: "s_nonexistent", Args: []string{"-s", f.nonExist}, ExitCode: 1},
		{Name: "r_readable", Args: []string{"-r", f.regFile}, ExitCode: 0},
		{Name: "r_nonexistent", Args: []string{"-r", f.nonExist}, ExitCode: 1},
		{Name: "w_writable", Args: []string{"-w", f.regFile}, ExitCode: 0},
		{Name: "w_nonexistent", Args: []string{"-w", f.nonExist}, ExitCode: 1},
		{Name: "x_executable", Args: []string{"-x", f.execFile}, ExitCode: 0},
		{Name: "x_not_exec", Args: []string{"-x", f.regFile}, ExitCode: 1},
		{Name: "L_symlink", Args: []string{"-L", f.symlink}, ExitCode: 0},
		{Name: "L_regular", Args: []string{"-L", f.regFile}, ExitCode: 1},
		{Name: "L_broken_link", Args: []string{"-L", f.brokenLink}, ExitCode: 0},
		{Name: "h_symlink", Args: []string{"-h", f.symlink}, ExitCode: 0},
		{Name: "h_regular", Args: []string{"-h", f.regFile}, ExitCode: 1},
		{Name: "p_fifo", Args: []string{"-p", f.fifo}, ExitCode: 0},
		{Name: "p_regular", Args: []string{"-p", f.regFile}, ExitCode: 1},
	}
}

// fileCompareTests returns differential tests for -nt, -ot, -ef operators (R3.2).
func fileCompareTests(f testFixtures) []testutils.DiffTest {
	return []testutils.DiffTest{
		{Name: "nt_newer", Args: []string{f.newerFile, "-nt", f.olderFile}, ExitCode: 0},
		{Name: "nt_older", Args: []string{f.olderFile, "-nt", f.newerFile}, ExitCode: 1},
		{Name: "nt_exist_vs_nonexist", Args: []string{f.regFile, "-nt", f.nonExist}, ExitCode: 0},
		{Name: "nt_nonexist_vs_exist", Args: []string{f.nonExist, "-nt", f.regFile}, ExitCode: 1},
		{Name: "ot_older", Args: []string{f.olderFile, "-ot", f.newerFile}, ExitCode: 0},
		{Name: "ot_newer", Args: []string{f.newerFile, "-ot", f.olderFile}, ExitCode: 1},
		{Name: "ef_hardlink", Args: []string{f.regFile, "-ef", f.hardLink}, ExitCode: 0},
		{Name: "ef_different", Args: []string{f.regFile, "-ef", f.emptyFile}, ExitCode: 1},
		{Name: "ef_nonexist", Args: []string{f.regFile, "-ef", f.nonExist}, ExitCode: 1},
		{Name: "ef_symlink", Args: []string{f.symlink, "-ef", f.regFile}, ExitCode: 0},
	}
}

// errorTests returns differential tests for error handling (R4.1).
func errorTests() []testutils.DiffTest {
	norm := []testutils.NormalizeFunc{stderrNormalizer}
	return []testutils.DiffTest{
		{
			Name: "err_missing_paren", Args: []string{"(", "foo"},
			ExitCode: 2, Normalize: norm,
		},
		{
			Name: "err_extra_arg", Args: []string{"1", "-eq", "2", "extra"},
			ExitCode: 2, Normalize: norm,
		},
	}
}
