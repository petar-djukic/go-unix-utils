// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ln comparing against gln (GNU coreutils).
// Covers prd037-ln R1.1-R1.4 (hard links), R2.1-R2.2 (symbolic links).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// cmdResult holds captured output from a binary invocation.
type cmdResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// stderrNormalizer normalizes error messages between GNU gln and Go ln.
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?ln|gln`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("ln"))
		b = tryHelp.ReplaceAll(b, nil)
		b = bytes.ToLower(b)
		return b
	}
}

// TestDiffErrors tests error cases that don't mutate filesystem state,
// using RunDiffTests directly.
func TestDiffErrors(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	// R1.3: create a directory for hard-link-to-directory test.
	dirPath := filepath.Join(t.TempDir(), "testdir")
	if mkErr := os.Mkdir(dirPath, 0o755); mkErr != nil {
		t.Fatalf("create test directory: %v", mkErr)
	}

	tests := []testutils.DiffTest{
		// R1.4: missing operand
		{
			Name:      "no_args",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.3: hard link to directory fails
		{
			Name:      "hard_link_to_directory",
			Args:      []string{dirPath, filepath.Join(t.TempDir(), "link_to_dir")},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Error: nonexistent target for hard link
		{
			Name:      "hard_link_nonexistent",
			Args:      []string{"no_such_file_xyz", "link_xyz"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runBin runs a binary with args in the given working directory.
func runBin(t *testing.T, binary string, args []string, dir string) cmdResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, runErr)
		}
	}
	return cmdResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// compareResults asserts that ref and go binary outputs match.
func compareResults(t *testing.T, name string, ref, got cmdResult) {
	t.Helper()
	norm := stderrNormalizer()
	if !bytes.Equal(ref.stdout, got.stdout) {
		t.Errorf("[%s] stdout mismatch\n  ref: %q\n  go:  %q",
			name, ref.stdout, got.stdout)
	}
	refErr := norm(ref.stderr)
	goErr := norm(got.stderr)
	if !bytes.Equal(refErr, goErr) {
		t.Errorf("[%s] stderr mismatch\n  ref: %q\n  go:  %q",
			name, refErr, goErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("[%s] exit code mismatch: ref=%d go=%d",
			name, ref.exitCode, got.exitCode)
	}
}

// writeFile creates a file with given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// mustMkdir creates a directory.
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// TestHardLinkBasic verifies R1.1: hard link from TARGET to LINK_NAME.
func TestHardLinkBasic(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "source.txt"), "hello")
	refRes := runBin(t, refBin, []string{"source.txt", "hardlink.txt"}, refDir)
	assertHardLink(t, filepath.Join(refDir, "source.txt"),
		filepath.Join(refDir, "hardlink.txt"), "ref")

	// Go binary
	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "source.txt"), "hello")
	goRes := runBin(t, goBin, []string{"source.txt", "hardlink.txt"}, goDir)
	assertHardLink(t, filepath.Join(goDir, "source.txt"),
		filepath.Join(goDir, "hardlink.txt"), "go")

	compareResults(t, "hard_link_basic", refRes, goRes)
}

// TestHardLinkMultipleIntoDir verifies R1.2: multiple targets into directory.
func TestHardLinkMultipleIntoDir(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "a.txt"), "aaa")
	writeFile(t, filepath.Join(refDir, "b.txt"), "bbb")
	mustMkdir(t, filepath.Join(refDir, "dest"))
	refRes := runBin(t, refBin, []string{"a.txt", "b.txt", "dest"}, refDir)
	assertHardLink(t, filepath.Join(refDir, "a.txt"),
		filepath.Join(refDir, "dest", "a.txt"), "ref-a")
	assertHardLink(t, filepath.Join(refDir, "b.txt"),
		filepath.Join(refDir, "dest", "b.txt"), "ref-b")

	// Go binary
	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "a.txt"), "aaa")
	writeFile(t, filepath.Join(goDir, "b.txt"), "bbb")
	mustMkdir(t, filepath.Join(goDir, "dest"))
	goRes := runBin(t, goBin, []string{"a.txt", "b.txt", "dest"}, goDir)
	assertHardLink(t, filepath.Join(goDir, "a.txt"),
		filepath.Join(goDir, "dest", "a.txt"), "go-a")
	assertHardLink(t, filepath.Join(goDir, "b.txt"),
		filepath.Join(goDir, "dest", "b.txt"), "go-b")

	compareResults(t, "hard_link_multi", refRes, goRes)
}

// TestHardLinkExistingDest verifies R1.4: error when dest exists.
func TestHardLinkExistingDest(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "src")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "dst")
	refRes := runBin(t, refBin, []string{"src.txt", "dst.txt"}, refDir)

	// Go binary
	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "src")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "dst")
	goRes := runBin(t, goBin, []string{"src.txt", "dst.txt"}, goDir)

	compareResults(t, "hard_link_existing", refRes, goRes)
}

// TestSymlinkBasic verifies R2.1: -s creates symbolic link.
func TestSymlinkBasic(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "target.txt"), "content")
	refRes := runBin(t, refBin, []string{"-s", "target.txt", "sym.txt"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "sym.txt"), "target.txt", "ref")

	// Go binary
	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "target.txt"), "content")
	goRes := runBin(t, goBin, []string{"-s", "target.txt", "sym.txt"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "sym.txt"), "target.txt", "go")

	compareResults(t, "symlink_basic", refRes, goRes)
}

// TestSymlinkToDirectory verifies R2.2: symbolic links to directories.
func TestSymlinkToDirectory(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	mustMkdir(t, filepath.Join(refDir, "mydir"))
	refRes := runBin(t, refBin, []string{"-s", "mydir", "dirlink"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "dirlink"), "mydir", "ref")

	// Go binary
	goDir := t.TempDir()
	mustMkdir(t, filepath.Join(goDir, "mydir"))
	goRes := runBin(t, goBin, []string{"-s", "mydir", "dirlink"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "dirlink"), "mydir", "go")

	compareResults(t, "symlink_to_dir", refRes, goRes)
}

// TestSymlinkDangling verifies R2.1: dangling symlink (nonexistent target).
func TestSymlinkDangling(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	refRes := runBin(t, refBin,
		[]string{"-s", "nonexistent", "dangling"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "dangling"), "nonexistent", "ref")

	// Go binary
	goDir := t.TempDir()
	goRes := runBin(t, goBin,
		[]string{"-s", "nonexistent", "dangling"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "dangling"), "nonexistent", "go")

	compareResults(t, "symlink_dangling", refRes, goRes)
}

// assertHardLink verifies two paths share the same inode.
func assertHardLink(t *testing.T, src, dst, label string) {
	t.Helper()
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Errorf("[%s] stat source: %v", label, err)
		return
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Errorf("[%s] stat dest: %v", label, err)
		return
	}
	if !os.SameFile(srcInfo, dstInfo) {
		t.Errorf("[%s] %s and %s are not the same file", label, src, dst)
	}
}

// assertSymlink verifies a symlink points to the expected target.
func assertSymlink(t *testing.T, link, expectedTarget, label string) {
	t.Helper()
	target, err := os.Readlink(link)
	if err != nil {
		t.Errorf("[%s] readlink %s: %v", label, link, err)
		return
	}
	if target != expectedTarget {
		t.Errorf("[%s] symlink target = %q, want %q",
			label, target, expectedTarget)
	}
}
