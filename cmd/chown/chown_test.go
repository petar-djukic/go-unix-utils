// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/chown against gchown (GNU coreutils).
//
// Traces: prd091-chown R1.1, R1.2, R1.3, R1.4.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normProgName normalizes the program name prefix in stderr output so
// "gchown:" and "chown:" compare as equal.
func normProgName(b []byte) []byte {
	s := strings.ReplaceAll(string(b), "gchown:", "chown:")
	return []byte(s)
}

// currentUser returns the current user's username.
func currentUser(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("get current user: %v", err)
	}
	return u.Username
}

// currentUID returns the current user's UID string.
func currentUID(t *testing.T) string {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Fatalf("get current user: %v", err)
	}
	return u.Uid
}

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

// TestDiff runs differential tests comparing our chown against gchown.
func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gchown")
	if err != nil {
		t.Skip("reference binary gchown not in PATH")
	}

	owner := currentUser(t)
	uid := currentUID(t)
	group := currentGroup(t)
	gid := currentGID(t)
	ownerGroup := owner + ":" + group

	tests := []testutils.DiffTest{
		// R1.1: OWNER form — change owner by name
		{
			Name:    "owner_by_name",
			Args:    []string{owner, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.1: :GROUP form — change group only
		{
			Name:    "group_only",
			Args:    []string{":" + group, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.1: OWNER:GROUP form — change both
		{
			Name:    "owner_and_group",
			Args:    []string{ownerGroup, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.1: OWNER: form — change owner, set group to login group
		{
			Name:    "owner_colon_login_group",
			Args:    []string{owner + ":", "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.2: numeric UID
		{
			Name:    "numeric_uid",
			Args:    []string{uid, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.2: numeric UID:GID
		{
			Name:    "numeric_uid_gid",
			Args:    []string{uid + ":" + gid, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.3: --reference mode
		{
			Name:    "reference_mode",
			Args:    []string{"--reference=reffile", "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeRefWorkDir(t),
		},
		// R1.1: multiple files
		{
			Name:    "multiple_files",
			Args:    []string{ownerGroup, "file1", "file2"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeMultiWorkDir(t),
		},
		// R1.1: -- separator
		{
			Name:    "double_dash_separator",
			Args:    []string{"--", ownerGroup, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.4: exit 0 on success
		{
			Name:    "exit_zero_success",
			Args:    []string{owner, "testfile"},
			Env:     []string{"LC_ALL=C"},
			WorkDir: makeWorkDir(t),
		},
		// R1.4: exit 1 on nonexistent file
		{
			Name:      "exit_one_nonexistent",
			Args:      []string{owner, "noexist"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   t.TempDir(),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName},
		},
		// R1.4: exit 1 on invalid user
		{
			Name:      "exit_one_invalid_user",
			Args:      []string{"nonexistent_user_xyz_999", "testfile"},
			Env:       []string{"LC_ALL=C"},
			WorkDir:   makeWorkDir(t),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normProgName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestOwnerChange verifies owner is actually changed.
// R1.1: OWNER form.
func TestOwnerChange(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	owner := currentUser(t)
	dir := makeWorkDir(t)

	cmd := exec.Command(goBin, owner, "testfile")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chown failed: %v", err)
	}

	fi, err := sys.Lstat(filepath.Join(dir, "testfile"))
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}

	u, _ := user.Current()
	if fmt.Sprint(fi.Uid) != u.Uid {
		t.Errorf("uid = %d, want %s", fi.Uid, u.Uid)
	}
}

// TestOwnerGroupChange verifies both owner and group are changed.
// R1.1: OWNER:GROUP form.
func TestOwnerGroupChange(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	owner := currentUser(t)
	group := currentGroup(t)
	dir := makeWorkDir(t)

	cmd := exec.Command(goBin, owner+":"+group, "testfile")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chown owner:group failed: %v", err)
	}

	fi, err := sys.Lstat(filepath.Join(dir, "testfile"))
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}

	u, _ := user.Current()
	if fmt.Sprint(fi.Uid) != u.Uid {
		t.Errorf("uid = %d, want %s", fi.Uid, u.Uid)
	}
	if fmt.Sprint(fi.Gid) != u.Gid {
		t.Errorf("gid = %d, want %s", fi.Gid, u.Gid)
	}
}

// TestGroupOnlyChange verifies :GROUP form changes only group.
// R1.1: :GROUP form.
func TestGroupOnlyChange(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	group := currentGroup(t)
	dir := makeWorkDir(t)

	cmd := exec.Command(goBin, ":"+group, "testfile")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chown :group failed: %v", err)
	}

	fi, err := sys.Lstat(filepath.Join(dir, "testfile"))
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}

	u, _ := user.Current()
	if fmt.Sprint(fi.Gid) != u.Gid {
		t.Errorf("gid = %d, want %s", fi.Gid, u.Gid)
	}
}

// TestOwnerColonForm verifies OWNER: sets group to login group.
// R1.1: OWNER: form.
func TestOwnerColonForm(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	owner := currentUser(t)
	dir := makeWorkDir(t)

	cmd := exec.Command(goBin, owner+":", "testfile")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chown owner: failed: %v", err)
	}

	fi, err := sys.Lstat(filepath.Join(dir, "testfile"))
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}

	u, _ := user.Current()
	if fmt.Sprint(fi.Uid) != u.Uid {
		t.Errorf("uid = %d, want %s", fi.Uid, u.Uid)
	}
	// OWNER: form sets group to owner's login group
	if fmt.Sprint(fi.Gid) != u.Gid {
		t.Errorf("gid = %d, want %s (login group)", fi.Gid, u.Gid)
	}
}

// TestReferenceMode verifies --reference copies owner and group.
// R1.3: --reference=RFILE.
func TestReferenceMode(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := makeRefWorkDir(t)

	cmd := exec.Command(goBin, "--reference=reffile", "testfile")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chown --reference failed: %v", err)
	}

	refInfo, err := sys.Lstat(filepath.Join(dir, "reffile"))
	if err != nil {
		t.Fatalf("lstat reffile: %v", err)
	}
	testInfo, err := sys.Lstat(filepath.Join(dir, "testfile"))
	if err != nil {
		t.Fatalf("lstat testfile: %v", err)
	}
	if testInfo.Uid != refInfo.Uid {
		t.Errorf("testfile uid = %d, want %d (reffile)", testInfo.Uid, refInfo.Uid)
	}
	if testInfo.Gid != refInfo.Gid {
		t.Errorf("testfile gid = %d, want %d (reffile)", testInfo.Gid, refInfo.Gid)
	}
}

// TestMultipleFiles verifies ownership change applies to all files.
// R1.4: process multiple FILE arguments.
func TestMultipleFiles(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	owner := currentUser(t)
	group := currentGroup(t)
	dir := makeMultiWorkDir(t)

	cmd := exec.Command(goBin, owner+":"+group, "file1", "file2")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if err := cmd.Run(); err != nil {
		t.Fatalf("chown multiple files failed: %v", err)
	}

	u, _ := user.Current()
	for _, name := range []string{"file1", "file2"} {
		fi, err := sys.Lstat(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("lstat %s: %v", name, err)
		}
		if fmt.Sprint(fi.Uid) != u.Uid {
			t.Errorf("%s uid = %d, want %s", name, fi.Uid, u.Uid)
		}
		if fmt.Sprint(fi.Gid) != u.Gid {
			t.Errorf("%s gid = %d, want %s", name, fi.Gid, u.Gid)
		}
	}
}

// TestExitCodes verifies exit code behavior.
// R1.4: exit 1 on error, continue processing remaining files.
func TestExitCodes(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	owner := currentUser(t)

	t.Run("success_exits_zero", func(t *testing.T) {
		dir := makeWorkDir(t)
		cmd := exec.Command(goBin, owner, "testfile")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if err := cmd.Run(); err != nil {
			t.Errorf("expected exit 0, got: %v", err)
		}
	})

	t.Run("nonexistent_exits_one", func(t *testing.T) {
		dir := t.TempDir()
		cmd := exec.Command(goBin, owner, "noexist")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		err := cmd.Run()
		if err == nil {
			t.Error("expected exit 1, got exit 0")
		}
	})

	t.Run("invalid_user_exits_one", func(t *testing.T) {
		dir := makeWorkDir(t)
		cmd := exec.Command(goBin, "nonexistent_user_xyz_999", "testfile")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		err := cmd.Run()
		if err == nil {
			t.Error("expected exit 1, got exit 0")
		}
	})

	t.Run("partial_failure_exits_one", func(t *testing.T) {
		dir := makeWorkDir(t)
		cmd := exec.Command(goBin, owner, "testfile", "noexist")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		err := cmd.Run()
		if err == nil {
			t.Error("expected exit 1 for partial failure, got exit 0")
		}
	})
}
