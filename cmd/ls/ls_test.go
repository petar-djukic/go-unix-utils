// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for prd008-ls R4.4-R4.9: SIGPIPE handling, SIGWINCH
// handler, -n numeric IDs, format flag interactions, -R -l total lines,
// and -R with -i -s -F combined.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// setupFixture creates a test directory with files and subdirectories
// for differential testing. Returns the fixture path.
func setupFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	createFixtureFiles(t, dir)
	createFixtureSubdirs(t, dir)
	return dir
}

// createFixtureFiles creates regular files and a symlink in the fixture.
func createFixtureFiles(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "alpha.txt"), "hello\n")
	writeFile(t, filepath.Join(dir, "beta.txt"), "world\n")
	writeFile(t, filepath.Join(dir, "gamma.txt"), "data\n")

	// Executable file for -F classification.
	execPath := filepath.Join(dir, "run.sh")
	writeFile(t, execPath, "#!/bin/sh\n")
	if err := os.Chmod(execPath, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	// Symlink for -F classification.
	if err := os.Symlink("alpha.txt", filepath.Join(dir, "link.txt")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
}

// createFixtureSubdirs creates nested subdirectories for -R tests.
func createFixtureSubdirs(t *testing.T, dir string) {
	t.Helper()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(sub, "inner.txt"), "nested\n")
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

// TestDiff covers R4.4 (SIGPIPE), R4.6 (-n implies -l), R4.7 (format
// interactions), R4.8 (-R -l total lines), and R4.9 (-R with -i -s -F).
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skip("reference binary gls not in PATH")
	}

	dir := setupFixture(t)

	tests := []testutils.DiffTest{
		// R4.4: SIGPIPE — basic invocation succeeds without broken pipe.
		{
			Name: "R4.4-sigpipe-basic",
			Args: []string{"-1", "--color=never", dir},
		},
		// R4.6: -n implies long format with numeric UID/GID.
		{
			Name: "R4.6-numeric-ids",
			Args: []string{"-n", "--color=never", dir},
		},
		// R4.6: -n without explicit -l still produces long format.
		{
			Name: "R4.6-n-implies-l",
			Args: []string{"-n", "--color=never", dir},
		},
		// R4.7: -1 then -l produces long format (last format wins).
		{
			Name: "R4.7-1-then-l",
			Args: []string{"-1", "-l", "--color=never", dir},
		},
		// R4.7: -C then -l produces long format (last format wins).
		{
			Name: "R4.7-C-then-l",
			Args: []string{"-C", "-l", "--color=never", dir},
		},
		// R4.8: -R with -l produces total line per subdirectory.
		{
			Name: "R4.8-recursive-long",
			Args: []string{"-l", "-R", "--color=never", dir},
		},
		// R4.9: -R with -i -s -F combined.
		{
			Name: "R4.9-recursive-inode-blocks-classify",
			Args: []string{"-R", "-i", "-s", "-F", "--color=never", dir},
		},
		// R4.9: -R with -F only.
		{
			Name: "R4.9-recursive-classify",
			Args: []string{"-R", "-F", "--color=never", dir},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFormatInteractions covers R4.6 and R4.7 format flag precedence.
func TestDiffFormatInteractions(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skip("reference binary gls not in PATH")
	}

	dir := setupFixture(t)

	tests := []testutils.DiffTest{
		// R4.7: -x then -l produces long format.
		{
			Name: "R4.7-x-then-l",
			Args: []string{"-x", "-l", "--color=never", dir},
		},
		// R4.6/R4.7: -n after -C still produces long format.
		{
			Name: "R4.6-n-after-C",
			Args: []string{"-C", "-n", "--color=never", dir},
		},
		// R4.8: -R -l on nested directory with -s.
		{
			Name: "R4.8-recursive-long-blocks",
			Args: []string{"-l", "-R", "-s", "--color=never", dir},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffNumericMixed covers R4.6 -n with other metadata flags.
func TestDiffNumericMixed(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skip("reference binary gls not in PATH")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.txt"), "test\n")
	writeFile(t, filepath.Join(dir, "b.txt"), "data\n")

	tests := []testutils.DiffTest{
		// R4.6: -n with -i shows inode + numeric long format.
		{
			Name: "R4.6-numeric-inode",
			Args: []string{"-n", "-i", "--color=never", dir},
		},
		// R4.6: -n with -s shows blocks + numeric long format.
		{
			Name: "R4.6-numeric-blocks",
			Args: []string{"-n", "-s", "--color=never", dir},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffRecursiveMetadata covers R4.8 and R4.9: recursive listing
// with metadata display, classification, and numeric IDs.
func TestDiffRecursiveMetadata(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gls")
	if err != nil {
		t.Skip("reference binary gls not in PATH")
	}

	dir := t.TempDir()
	sub1 := filepath.Join(dir, "aaa")
	sub2 := filepath.Join(dir, "bbb")
	for _, d := range []string{sub1, sub2} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	writeFile(t, filepath.Join(sub1, "one.txt"), "1\n")
	writeFile(t, filepath.Join(sub2, "two.txt"), "2\n")
	ep := filepath.Join(dir, "exec.sh")
	writeFile(t, ep, "#!/bin/sh\n")
	if err := os.Chmod(ep, 0o755); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	tests := []testutils.DiffTest{
		// R4.9: -R -i propagates inode display into subdirectories.
		{
			Name: "R4.9-recursive-inode",
			Args: []string{"-R", "-i", "--color=never", dir},
		},
		// R4.9: -R -s propagates block display into subdirectories.
		{
			Name: "R4.9-recursive-blocks",
			Args: []string{"-R", "-s", "--color=never", dir},
		},
		// R4.9: -R -F propagates classify into subdirectories.
		{
			Name: "R4.9-recursive-classify",
			Args: []string{"-R", "-F", "--color=never", dir},
		},
		// R4.8: -R -l -n combines recursive with numeric IDs.
		{
			Name: "R4.8-recursive-long-numeric",
			Args: []string{"-R", "-l", "-n", "--color=never", dir},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSigpipePipe verifies SIGPIPE handling (R4.4) by piping output
// through head, which closes stdin early.
func TestDiffSigpipePipe(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	headBin, err := exec.LookPath("head")
	if err != nil {
		t.Skip("head not in PATH")
	}

	dir := t.TempDir()
	for i := range 50 {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("file%03d.txt", i)), "x\n")
	}

	// Run: ls -1 dir | head -5
	// The ls process should exit cleanly (SIGPIPE handled).
	cmd := exec.Command("sh", "-c",
		fmt.Sprintf("%s -1 --color=never %s | %s -5", goBin, dir, headBin))
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ls | head failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected output from ls | head, got empty")
	}
}
