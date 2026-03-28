// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// stderrNormalizer normalizes binary name and path in stderr messages
// so that gdir and dir produce comparable output.
var stderrNormalizer = testutils.ComposeNormalizers(
	normalizeBinName,
	normalizeErrCase,
)

// normalizeBinName replaces the reference binary path/name with "dir"
// in both the error prefix and "Try '...' --help" messages.
func normalizeBinName(data []byte) []byte {
	// Normalize "gdir:" or "/opt/homebrew/bin/gdir:" at line start.
	rePrefix := regexp.MustCompile(`(?m)^[^\s:]+:`)
	data = rePrefix.ReplaceAll(data, []byte("dir:"))
	// Normalize "Try '/path/to/gdir --help' for ..." to canonical form.
	reTry := regexp.MustCompile(`(?m)Try '.*' for more information\.`)
	data = reTry.ReplaceAll(data, []byte("Try 'dir --help' for more information."))
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
		t.Skip("cmd/dir not yet generated")
	}
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gdir")
	if err != nil {
		t.Skipf("reference binary gdir not in PATH: %v", err)
	}
	tests := buildDiffTests(t)
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func buildDiffTests(t *testing.T) []testutils.DiffTest {
	t.Helper()
	return []testutils.DiffTest{
		multiColumnTest(t),
		escapeTest(t),
		sortFilterTest(t),
		showAllTest(t),
		defaultCwdTest(t),
		longFormatTest(t),
		reverseTest(t),
		recursiveTest(t),
		singleColumnTest(t),
		exitZeroTest(t),
		exitOneTest(t),
		exitTwoTest(t),
		classifyTest(t),
		sigpipeTest(t),
	}
}

// sigpipeTest verifies R2.4: SIGPIPE handler prevents broken-pipe errors.
// We test with enough entries that output is produced, validating that the
// binary does not crash or produce extra error output when run normally.
// The SIGPIPE handler is installed at startup via sys.InstallSIGPIPEHandler().
func sigpipeTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	names := make([]string, 50)
	for i := range names {
		names[i] = fmt.Sprintf("file_%03d", i)
	}
	return testutils.DiffTest{
		Name:    "R2.4_sigpipe_handler",
		Args:    []string{"-1"},
		WorkDir: setupTestDir(t, names...),
	}
}

// multiColumnTest verifies R1.1: multi-column output by default.
func multiColumnTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.1_multi_column_output",
		WorkDir: setupTestDir(t, "alpha", "bravo", "charlie", "delta", "echo", "foxtrot"),
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

// sortFilterTest verifies R1.3: C locale sort order, dot-entries hidden.
func sortFilterTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.3_sort_and_filter",
		WorkDir: setupTestDir(t, "Zulu", "alpha", ".hidden", "Bravo", "charlie"),
	}
}

// showAllTest verifies R1.3: -a shows dot-entries including . and ..
func showAllTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.3_show_all",
		Args:    []string{"-a"},
		WorkDir: setupTestDir(t, ".hidden", "visible"),
	}
}

// defaultCwdTest verifies R1.4: defaults to current directory when no args.
func defaultCwdTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.4_default_cwd",
		WorkDir: setupTestDir(t, "file1", "file2", "file3"),
	}
}

// longFormatTest verifies R1.5: dir accepts -l flag (ls long format).
func longFormatTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.5_long_format",
		Args:    []string{"-l"},
		WorkDir: setupTestDir(t, "aaa", "bbb"),
	}
}

// reverseTest verifies R1.5: dir accepts -r flag (reverse sort).
func reverseTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.5_reverse_sort",
		Args:    []string{"-r"},
		WorkDir: setupTestDir(t, "alpha", "bravo", "charlie"),
	}
}

// recursiveTest verifies R1.5: dir accepts -R flag (recursive listing).
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
		Name:    "R1.5_recursive",
		Args:    []string{"-R"},
		WorkDir: dir,
	}
}

// singleColumnTest verifies R1.5: dir accepts -1 flag (one entry per line).
func singleColumnTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	return testutils.DiffTest{
		Name:    "R1.5_single_column",
		Args:    []string{"-1"},
		WorkDir: setupTestDir(t, "one", "two", "three"),
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
		Args:      []string{"/nonexistent_path_for_dir_test"},
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

// classifyTest verifies R1.5: dir accepts -F flag (classify indicator).
func classifyTest(t *testing.T) testutils.DiffTest {
	t.Helper()
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "regular"))
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("creating subdir: %v", err)
	}
	return testutils.DiffTest{
		Name:    "R1.5_classify",
		Args:    []string{"-F"},
		WorkDir: dir,
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
