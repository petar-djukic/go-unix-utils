// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ln covering prd037-ln R1.1–R1.4 (hard links)
// and R2.1–R2.4 (symbolic links).
package main_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	normBin := makeBinaryNormalizer(refBin)
	runSharedTests(t, goBin, refBin, normBin)
	runIsolatedTests(t, goBin, refBin, normBin)
}

// runSharedTests runs tests where both binaries can share the same WorkDir
// (error cases where no filesystem mutation succeeds).
func runSharedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()

	dirWithFile := t.TempDir()
	setupFile(t, dirWithFile, "source.txt", "hello")
	setupFile(t, dirWithFile, "existing.txt", "world")

	dirWithSubdir := t.TempDir()
	requireMkdir(t, filepath.Join(dirWithSubdir, "subdir"))

	dirWithSymSrc := t.TempDir()
	setupFile(t, dirWithSymSrc, "target.txt", "data")
	setupFile(t, dirWithSymSrc, "existing.txt", "old")

	tests := []testutils.DiffTest{
		// R1.3: hard link to directory rejected.
		{
			Name:      "hard_link_dir_rejected",
			Args:      []string{filepath.Join(dirWithSubdir, "subdir"), filepath.Join(dirWithSubdir, "link")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
		// R1.4: existing destination without -f.
		{
			Name:      "hard_link_existing",
			Args:      []string{filepath.Join(dirWithFile, "source.txt"), filepath.Join(dirWithFile, "existing.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
		// R2.1 + R1.4: symlink to existing file fails.
		{
			Name:      "symlink_existing_dest",
			Args:      []string{"-s", filepath.Join(dirWithSymSrc, "target.txt"), filepath.Join(dirWithSymSrc, "existing.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
		// Missing operand.
		{
			Name:      "missing_operand",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normBin},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a test where each binary runs in its own temp dir.
type isolatedCase struct {
	name  string
	args  []string // uses placeholder paths; setup provides the real files
	setup func(t *testing.T, dir string)
	norm  []testutils.NormalizeFunc
	// verify runs after both binaries complete; checks filesystem state.
	verify func(t *testing.T, goDir string)
}

// runIsolatedTests runs tests where each binary needs its own WorkDir
// because both create links.
func runIsolatedTests(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		// R1.1: create a hard link.
		{
			name: "hard_link_single",
			args: []string{"source.txt", "link.txt"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "source.txt", "hello")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyHardLink(t, filepath.Join(dir, "source.txt"), filepath.Join(dir, "link.txt"))
			},
		},
		// R1.2: multiple targets into directory.
		{
			name: "hard_link_multiple_into_dir",
			args: []string{"a.txt", "b.txt", "dest"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "a.txt", "aaa")
				setupFile(t, dir, "b.txt", "bbb")
				requireMkdir(t, filepath.Join(dir, "dest"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifyHardLink(t, filepath.Join(dir, "a.txt"), filepath.Join(dir, "dest", "a.txt"))
				verifyHardLink(t, filepath.Join(dir, "b.txt"), filepath.Join(dir, "dest", "b.txt"))
			},
		},
		// R2.1: -s creates symbolic link.
		{
			name: "symlink_single",
			args: []string{"-s", "target.txt", "slink"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "target.txt", "content")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifySymlink(t, filepath.Join(dir, "slink"), "target.txt")
			},
		},
		// R2.1: --symbolic long form.
		{
			name: "symlink_long_flag",
			args: []string{"--symbolic", "target.txt", "slink"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "target.txt", "data")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifySymlink(t, filepath.Join(dir, "slink"), "target.txt")
			},
		},
		// R2.2: symbolic links to directories allowed.
		{
			name: "symlink_to_directory",
			args: []string{"-s", "subdir", "dirlink"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "subdir"))
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifySymlink(t, filepath.Join(dir, "dirlink"), "subdir")
			},
		},
		// R2.3: relative target stored as-is.
		{
			name: "symlink_relative_asis",
			args: []string{"-s", "target.txt", "slink"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "target.txt", "data")
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifySymlink(t, filepath.Join(dir, "slink"), "target.txt")
			},
		},
		// R2.3: absolute target stored as-is.
		{
			name: "symlink_absolute_asis",
			args: []string{"-s", "/usr/bin/env", "envlink"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
			},
			verify: func(t *testing.T, dir string) {
				t.Helper()
				verifySymlink(t, filepath.Join(dir, "envlink"), "/usr/bin/env")
			},
		},
		// R2.4: -r creates relative symlink from absolute paths.
		{
			name: "symlink_relative_flag",
			args: nil, // set dynamically
			setup: func(t *testing.T, dir string) {
				t.Helper()
				setupFile(t, dir, "target.txt", "data")
			},
		},
		// R2.4: -sr combined short flags with subdirectory.
		{
			name: "symlink_combined_sr_subdir",
			args: nil, // set dynamically
			setup: func(t *testing.T, dir string) {
				t.Helper()
				requireMkdir(t, filepath.Join(dir, "sub"))
				setupFile(t, dir, "sub/deep.txt", "deep")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.name == "symlink_relative_flag" {
				runRelativeSymlinkTest(t, goBin, refBin, normBin)
				return
			}
			if tc.name == "symlink_combined_sr_subdir" {
				runRelativeSubdirTest(t, goBin, refBin, normBin)
				return
			}
			compareIsolated(t, goBin, refBin, tc)
		})
	}
}

// runRelativeSymlinkTest tests R2.4 with -r using absolute paths.
func runRelativeSymlinkTest(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	setupFile(t, refDir, "target.txt", "data")
	setupFile(t, goDir, "target.txt", "data")

	refArgs := []string{"-sr", filepath.Join(refDir, "target.txt"), filepath.Join(refDir, "rlink")}
	goArgs := []string{"-sr", filepath.Join(goDir, "target.txt"), filepath.Join(goDir, "rlink")}

	refOut, refErr, refCode := execBinary(t, refBin, refArgs, refDir)
	goOut, goErr, goCode := execBinary(t, goBin, goArgs, goDir)

	refOut, goOut, refErr, goErr = applyNorm(
		[]testutils.NormalizeFunc{normBin}, refOut, goOut, refErr, goErr)
	assertMatch(t, refArgs, refOut, goOut, refErr, goErr, refCode, goCode)
	verifySymlink(t, filepath.Join(goDir, "rlink"), "target.txt")
}

// runRelativeSubdirTest tests R2.4 with -sr and a subdirectory target.
func runRelativeSubdirTest(t *testing.T, goBin, refBin string, normBin testutils.NormalizeFunc) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	for _, d := range []string{refDir, goDir} {
		requireMkdir(t, filepath.Join(d, "sub"))
		setupFile(t, d, "sub/deep.txt", "deep")
	}

	refArgs := []string{"-sr", filepath.Join(refDir, "sub", "deep.txt"), filepath.Join(refDir, "toplink")}
	goArgs := []string{"-sr", filepath.Join(goDir, "sub", "deep.txt"), filepath.Join(goDir, "toplink")}

	refOut, refErr, refCode := execBinary(t, refBin, refArgs, refDir)
	goOut, goErr, goCode := execBinary(t, goBin, goArgs, goDir)

	refOut, goOut, refErr, goErr = applyNorm(
		[]testutils.NormalizeFunc{normBin}, refOut, goOut, refErr, goErr)
	assertMatch(t, refArgs, refOut, goOut, refErr, goErr, refCode, goCode)
	verifySymlink(t, filepath.Join(goDir, "toplink"), filepath.Join("sub", "deep.txt"))
}

// compareIsolated runs both binaries in separate temp dirs and compares.
func compareIsolated(t *testing.T, goBin, refBin string, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	if tc.setup != nil {
		tc.setup(t, refDir)
		tc.setup(t, goDir)
	}
	refOut, refErr, refCode := execBinary(t, refBin, tc.args, refDir)
	goOut, goErr, goCode := execBinary(t, goBin, tc.args, goDir)
	refOut, goOut, refErr, goErr = applyNorm(tc.norm, refOut, goOut, refErr, goErr)
	assertMatch(t, tc.args, refOut, goOut, refErr, goErr, refCode, goCode)
	if tc.verify != nil {
		tc.verify(t, goDir)
	}
}

// assertMatch fails the test if ref and go outputs diverge.
func assertMatch(t *testing.T, args []string, refOut, goOut, refErr, goErr []byte, refCode, goCode int) {
	t.Helper()
	if !bytes.Equal(refOut, goOut) || !bytes.Equal(refErr, goErr) || refCode != goCode {
		t.Fatalf("divergence\nargs:       %v\n"+
			"ref stdout: %s\ngo  stdout: %s\n"+
			"ref stderr: %s\ngo  stderr: %s\n"+
			"ref exit:   %d\ngo  exit:   %d",
			args, refOut, goOut, refErr, goErr, refCode, goCode)
	}
}

// execBinary runs a binary with args in dir, returning stdout, stderr,
// and exit code.
func execBinary(t *testing.T, bin string, args []string, dir string) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Env = buildTestEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", bin)
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return stdout.Bytes(), stderr.Bytes(), exitErr.ExitCode()
		}
		t.Fatalf("binary %s failed to execute: %v", bin, err)
	}
	return stdout.Bytes(), stderr.Bytes(), 0
}

// buildTestEnv returns the process environment with LC_ALL=C.
func buildTestEnv() []string {
	env := os.Environ()
	prefix := "LC_ALL="
	for i, e := range env {
		if len(e) >= len(prefix) && e[:len(prefix)] == prefix {
			env[i] = "LC_ALL=C"
			return env
		}
	}
	return append(env, "LC_ALL=C")
}

// makeBinaryNormalizer returns a normalizer that replaces the reference
// binary name with "ln" and lowercases to handle strerror differences.
func makeBinaryNormalizer(refBin string) testutils.NormalizeFunc {
	refDir := filepath.Dir(refBin)
	return func(data []byte) []byte {
		data = bytes.ReplaceAll(data, []byte(refBin), []byte("ln"))
		if refDir != "" {
			data = bytes.ReplaceAll(data, []byte(refDir+"/ln"), []byte("ln"))
		}
		data = bytes.ReplaceAll(data, []byte("gln"), []byte("ln"))
		return bytes.ToLower(data)
	}
}

// setupFile creates a file with the given content in dir.
func setupFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("setup file %s: %v", path, err)
	}
}

// requireMkdir creates a directory, failing the test on error.
func requireMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// verifySymlink checks that path is a symlink pointing to wantTarget.
func verifySymlink(t *testing.T, path, wantTarget string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	if got != wantTarget {
		t.Fatalf("symlink %s target = %q, want %q", path, got, wantTarget)
	}
}

// verifyHardLink checks that two paths share the same inode.
func verifyHardLink(t *testing.T, source, link string) {
	t.Helper()
	srcInfo, err := os.Stat(source)
	if err != nil {
		t.Fatalf("stat source %s: %v", source, err)
	}
	lnkInfo, err := os.Stat(link)
	if err != nil {
		t.Fatalf("stat link %s: %v", link, err)
	}
	if !os.SameFile(srcInfo, lnkInfo) {
		t.Fatalf("hard link %s and %s do not share inode", source, link)
	}
}

// applyNorm applies normalizers to ref and go stdout/stderr pairs.
func applyNorm(
	norm []testutils.NormalizeFunc,
	refOut, goOut, refErr, goErr []byte,
) ([]byte, []byte, []byte, []byte) {
	for _, n := range norm {
		refOut = n(refOut)
		goOut = n(goOut)
		refErr = n(refErr)
		goErr = n(goErr)
	}
	return refOut, goOut, refErr, goErr
}
