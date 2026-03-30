// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/chgrp against gchgrp (GNU coreutils).
//
// Traces: prd090-chgrp R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// currentGroup returns the current user's primary group name.
func currentGroup(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("get current user: %v", err)
	}
	grp, err := user.LookupGroupId(u.Gid)
	if err != nil {
		t.Fatalf("lookup group id %s: %v", u.Gid, err)
	}
	return grp.Name
}

// currentGID returns the current user's primary GID string.
func currentGID(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("get current user: %v", err)
	}
	return u.Gid
}

// setupFile creates a file in dir with specified permissions.
func setupFile(t *testing.T, dir, name string, perm os.FileMode) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("test\n"), perm); err != nil {
		t.Fatalf("setup: write %s: %v", name, err)
	}
}

// makeWorkDir creates a temp dir with a single testfile.
func makeWorkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	setupFile(t, dir, "testfile", 0o644)
	return dir
}

// makeMultiWorkDir creates a temp dir with two files.
func makeMultiWorkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	setupFile(t, dir, "file1", 0o644)
	setupFile(t, dir, "file2", 0o644)
	return dir
}

// makeRefWorkDir creates a temp dir with a testfile and a reffile.
func makeRefWorkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	setupFile(t, dir, "testfile", 0o644)
	setupFile(t, dir, "reffile", 0o644)
	return dir
}

// makeRecursiveDir creates a temp dir with a nested directory structure.
func makeRecursiveDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sub := filepath.Join(dir, "subdir")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatalf("setup: mkdir subdir: %v", err)
	}
	setupFile(t, dir, "topfile", 0o644)
	setupFile(t, sub, "subfile", 0o644)
	return dir
}

// makeSymlinkDir creates a temp dir with a symlink for testing -h/-H/-L/-P.
func makeSymlinkDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	setupFile(t, dir, "target", 0o644)
	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		t.Fatalf("setup: symlink: %v", err)
	}
	return dir
}

// TestDiff runs differential tests comparing our chgrp against gchgrp.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gchgrp")
	if err != nil {
		t.Skip("reference binary gchgrp not in PATH")
	}

	group := currentGroup(t)
	gid := currentGID(t)

	tests := []testutils.DiffTest{
		// R1.1: change group by name
		{
			Name:    "group_by_name",
			Args:    []string{group, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.1: change group by numeric GID
		{
			Name:    "group_by_gid",
			Args:    []string{gid, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.2: --reference mode
		{
			Name:    "reference_mode",
			Args:    []string{"--reference=reffile", "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeRefWorkDir(t),
		},
		// R1.3: multiple files
		{
			Name:    "multiple_files",
			Args:    []string{group, "file1", "file2"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeMultiWorkDir(t),
		},
		// R1.1: change group with -- separator
		{
			Name:    "double_dash_separator",
			Args:    []string{"--", group, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R2.1: recursive group change
		{
			Name:    "recursive_change",
			Args:    []string{"-R", group, "."},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeRecursiveDir(t),
		},
		// R2.2: no-dereference with -h
		{
			Name:    "no_dereference_h",
			Args:    []string{"-h", group, "link"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeSymlinkDir(t),
		},
		// R2.3: -P with recursive (default behavior)
		{
			Name:    "recursive_P",
			Args:    []string{"-R", "-P", group, "."},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeRecursiveDir(t),
		},
		// R3.1: verbose output
		{
			Name:    "verbose_output",
			Args:    []string{"-v", group, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R3.1: changes output (no change expected since group is same)
		{
			Name:    "changes_no_change",
			Args:    []string{"-c", group, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R3.1: verbose with recursive
		{
			Name:    "verbose_recursive",
			Args:    []string{"-Rv", group, "."},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeRecursiveDir(t),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestGroupChange verifies group ownership actually changes.
// R1.1: change file group to GROUP.
func TestGroupChange(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	group := currentGroup(t)
	dir := makeWorkDir(t)

	cmd := exec.Command(goBin, group, "testfile")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chgrp failed: %v", err)
	}

	fi, err := sys.Lstat(filepath.Join(dir, "testfile"))
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}

	u, _ := user.Current()
	wantGID := u.Gid
	gotGID := fi.Gid
	if wantGID != fmt.Sprint(gotGID) {
		t.Errorf("group = %d, want %s", gotGID, wantGID)
	}
}

// TestReferenceMode verifies --reference copies group from reference file.
// R1.2: --reference=RFILE sets each FILE's group to match RFILE's group.
func TestReferenceMode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := makeRefWorkDir(t)

	cmd := exec.Command(goBin, "--reference=reffile", "testfile")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chgrp --reference failed: %v", err)
	}

	refInfo, err := sys.Lstat(filepath.Join(dir, "reffile"))
	if err != nil {
		t.Fatalf("lstat reffile: %v", err)
	}
	testInfo, err := sys.Lstat(filepath.Join(dir, "testfile"))
	if err != nil {
		t.Fatalf("lstat testfile: %v", err)
	}
	if testInfo.Gid != refInfo.Gid {
		t.Errorf("testfile gid = %d, want %d (reffile)", testInfo.Gid, refInfo.Gid)
	}
}

// TestMultipleFiles verifies group change applies to all files.
// R1.3: process multiple FILE arguments.
func TestMultipleFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	group := currentGroup(t)
	dir := makeMultiWorkDir(t)

	cmd := exec.Command(goBin, group, "file1", "file2")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chgrp multiple files failed: %v", err)
	}

	for _, name := range []string{"file1", "file2"} {
		fi, err := sys.Lstat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("lstat %s: %v", name, err)
		}
		u, _ := user.Current()
		if fmt.Sprint(fi.Gid) != u.Gid {
			t.Errorf("%s gid = %d, want %s", name, fi.Gid, u.Gid)
		}
	}
}

// TestExitCodes verifies exit code behavior.
// R1.4: exit 1 on error, continue processing remaining files.
func TestExitCodes(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	group := currentGroup(t)

	t.Run("success_exits_zero", func(t *testing.T) {
		dir := makeWorkDir(t)
		cmd := exec.Command(goBin, group, "testfile")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if err := cmd.Run(); err != nil {
			t.Errorf("expected exit 0, got: %v", err)
		}
	})

	t.Run("nonexistent_exits_one", func(t *testing.T) {
		dir := t.TempDir()
		cmd := exec.Command(goBin, group, "noexist")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		err := cmd.Run()
		if err == nil {
			t.Error("expected exit 1, got exit 0")
		}
	})

	t.Run("invalid_group_exits_one", func(t *testing.T) {
		dir := makeWorkDir(t)
		cmd := exec.Command(goBin, "nonexistent_group_xyz_999", "testfile")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		err := cmd.Run()
		if err == nil {
			t.Error("expected exit 1, got exit 0")
		}
	})

	t.Run("partial_failure_exits_one", func(t *testing.T) {
		dir := makeWorkDir(t)
		cmd := exec.Command(goBin, group, "testfile", "noexist")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		err := cmd.Run()
		if err == nil {
			t.Error("expected exit 1 for partial failure, got exit 0")
		}
	})
}

// TestRecursiveChange verifies -R changes group recursively.
// R2.1: recursive group change.
func TestRecursiveChange(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	group := currentGroup(t)
	dir := makeRecursiveDir(t)

	cmd := exec.Command(goBin, "-R", group, dir)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chgrp -R failed: %v", err)
	}

	u, _ := user.Current()
	for _, rel := range []string{"", "topfile", "subdir", "subdir/subfile"} {
		path := filepath.Join(dir, rel)
		fi, err := sys.Lstat(path)
		if err != nil {
			t.Fatalf("lstat %s: %v", rel, err)
		}
		if fmt.Sprint(fi.Gid) != u.Gid {
			t.Errorf("%s gid = %d, want %s", rel, fi.Gid, u.Gid)
		}
	}
}

// TestNoDereference verifies -h changes the symlink, not the target.
// R2.2: --no-dereference.
func TestNoDereference(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	group := currentGroup(t)
	dir := makeSymlinkDir(t)

	cmd := exec.Command(goBin, "-h", group, "link")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chgrp -h failed: %v", err)
	}

	fi, err := sys.Lstat(filepath.Join(dir, "link"))
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	u, _ := user.Current()
	if fmt.Sprint(fi.Gid) != u.Gid {
		t.Errorf("link gid = %d, want %s", fi.Gid, u.Gid)
	}
}

// TestVerboseOutput verifies -v prints diagnostics.
// R3.1: verbose output.
func TestVerboseOutput(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	group := currentGroup(t)
	dir := makeWorkDir(t)

	cmd := exec.Command(goBin, "-v", group, "testfile")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("chgrp -v failed: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected verbose output, got empty")
	}
}

// TestChangesOutput verifies -c prints only when group changes.
// R3.1: changes output.
func TestChangesOutput(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	group := currentGroup(t)
	dir := makeWorkDir(t)

	// First run sets group to current (no change expected)
	cmd := exec.Command(goBin, "-c", group, "testfile")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("chgrp -c failed: %v", err)
	}
	// File already has our group, so -c should produce no output
	if len(out) != 0 {
		t.Errorf("expected no output for no-change, got %q", string(out))
	}
}
