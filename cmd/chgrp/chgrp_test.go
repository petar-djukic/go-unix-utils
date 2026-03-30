// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/chgrp against gchgrp (GNU coreutils).
//
// Traces: prd090-chgrp R1.1, R1.2, R1.3, R1.4.
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

