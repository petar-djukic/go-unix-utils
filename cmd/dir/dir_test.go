// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

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

// setupTestDir creates a temporary directory with the given filenames.
func setupTestDir(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatalf("creating test file %q: %v", name, err)
		}
	}
	return dir
}
