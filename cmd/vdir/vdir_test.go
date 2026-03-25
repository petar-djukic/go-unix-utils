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

// stderrNormalizer normalizes binary name and path in stderr messages
// so that gvdir and vdir produce comparable output.
var stderrNormalizer = testutils.ComposeNormalizers(
	normalizeBinName,
	normalizeErrCase,
)

// normalizeBinName replaces the reference binary path/name with "vdir"
// in both the error prefix and "Try '...' --help" messages.
func normalizeBinName(data []byte) []byte {
	// Normalize "gvdir:" or "/opt/homebrew/bin/gvdir:" at line start.
	rePrefix := regexp.MustCompile(`(?m)^[^\s:]+:`)
	data = rePrefix.ReplaceAll(data, []byte("vdir:"))
	// Normalize "Try '/path/to/gvdir --help' for ..." to canonical form.
	reTry := regexp.MustCompile(`(?m)Try '.*' for more information\.`)
	data = reTry.ReplaceAll(data, []byte("Try 'vdir --help' for more information."))
	return data
}

// normalizeErrCase lowercases common OS error messages for comparison.
func normalizeErrCase(data []byte) []byte {
	data = bytes.ReplaceAll(data, []byte("No such file or directory"), []byte("no such file or directory"))
	data = bytes.ReplaceAll(data, []byte("Permission denied"), []byte("permission denied"))
	data = bytes.ReplaceAll(data, []byte("Not a directory"), []byte("not a directory"))
	return data
}

func TestDiff(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat(filepath.Join(".", "main.go")); os.IsNotExist(err) {
		t.Skip("cmd/vdir not yet generated")
	}
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gvdir")
	if err != nil {
		t.Skipf("reference binary gvdir not in PATH: %v", err)
	}
	tests := buildDiffTests(t)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func buildDiffTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	return []testutils.DiffTest{
		longFormatDefaultTest(t),
		escapeTest(t),
		totalLineTest(t),
		sortFilterTest(t),
		showAllTest(t),
		defaultCwdTest(t),
		singleColumnOverrideTest(t),
		columnarOverrideTest(t),
		reverseTest(t),
		recursiveTest(t),
		exitZeroTest(t),
		exitOneTest(t),
		exitTwoTest(t),
	}
}

// longFormatDefaultTest verifies R1.1: long format output by default.
func longFormatDefaultTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.1_long_format_default",
		WorkDir: setupTestDir(t, "alpha", "bravo", "charlie"),
	}
}

// escapeTest verifies R1.2: C-style escaping of special characters.
func escapeTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.2_escape_special_chars",
		WorkDir: setupTestDir(t, "normal", "with space", "back\\slash"),
	}
}

// totalLineTest verifies R1.3: "total N" block count line.
func totalLineTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	// Create files with some content to produce non-zero block counts.
	for _, name := range []string{"aaa", "bbb", "ccc"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("content\n"), 0o644); err != nil {
			t.Fatalf("creating test file %q: %v", path, err)
		}
	}
	return testutils.DiffTest{
		Name:    "R1.3_total_line",
		WorkDir: dir,
	}
}

// sortFilterTest verifies R1.4: C locale sort order, dot-entries hidden.
func sortFilterTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.4_sort_and_filter",
		WorkDir: setupTestDir(t, "Zulu", "alpha", ".hidden", "Bravo", "charlie"),
	}
}

// showAllTest verifies -a shows dot-entries including . and ..
func showAllTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "show_all",
		Args:    []string{"-a"},
		WorkDir: setupTestDir(t, ".hidden", "visible"),
	}
}

// defaultCwdTest verifies R1.5: defaults to current directory when no args.
func defaultCwdTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.5_default_cwd",
		WorkDir: setupTestDir(t, "file1", "file2", "file3"),
	}
}

// singleColumnOverrideTest verifies R1.6: accepts -1 to override long format.
func singleColumnOverrideTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.6_single_column_override",
		Args:    []string{"-1"},
		WorkDir: setupTestDir(t, "one", "two", "three"),
	}
}

// columnarOverrideTest verifies R1.6: accepts -C to override long format.
func columnarOverrideTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.6_columnar_override",
		Args:    []string{"-C"},
		WorkDir: setupTestDir(t, "alpha", "bravo", "charlie", "delta"),
	}
}

// reverseTest verifies R1.6: accepts -r flag (reverse sort).
func reverseTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.6_reverse_sort",
		Args:    []string{"-r"},
		WorkDir: setupTestDir(t, "alpha", "bravo", "charlie"),
	}
}

// recursiveTest verifies R1.6: accepts -R flag (recursive listing).
func recursiveTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	for _, name := range []string{"top1", "top2"} {
		writeTestFile(t, filepath.Join(dir, name))
	}
	writeTestFile(t, filepath.Join(sub, "nested"))
	return testutils.DiffTest{
		Name:    "R1.6_recursive",
		Args:    []string{"-R"},
		WorkDir: dir,
	}
}

// exitZeroTest verifies R2.1: exit 0 on success.
func exitZeroTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R2.1_exit_zero_success",
		WorkDir: setupTestDir(t, "ok"),
	}
}

// exitOneTest verifies R2.2: exit code matches reference on minor error.
func exitOneTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:      "R2.2_exit_minor_error",
		Args:      []string{"/nonexistent_path_for_vdir_test"},
		Normalize: []testutils.NormalizeFunc{stderrNormalizer},
	}
}

// exitTwoTest verifies R2.3: exit 2 on serious error (invalid option).
func exitTwoTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:      "R2.3_exit_two_invalid_option",
		Args:      []string{"--invalid-option-xyz"},
		Normalize: []testutils.NormalizeFunc{stderrNormalizer},
	}
}

// setupTestDir creates a temporary directory with the given filenames.
func setupTestDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		writeTestFile(t, filepath.Join(dir, name))
	}
	return dir
}

// writeTestFile creates an empty file at the given path.
func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("creating test file %q: %v", path, err)
	}
}
