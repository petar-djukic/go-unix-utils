// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gchown")
	if err != nil {
		t.Skip("reference binary gchown not found")
	}

	cur, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}

	groups := testGroups(t)
	if len(groups) < 2 {
		t.Skip("need at least two groups for testing")
	}
	groupA := groups[0]
	groupB := groups[1]

	t.Run("colon_group", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "file"), []byte("x"), 0o644)
		setGroup(t, filepath.Join(workDir, "file"), groupA.gid)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "change_by_name",
				Args:    []string{":" + groupB.name, "file"},
				WorkDir: workDir,
			},
		})
		verifyGroup(t, filepath.Join(workDir, "file"), groupB.gid)
	})

	t.Run("colon_numeric_gid", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "file"), []byte("x"), 0o644)
		setGroup(t, filepath.Join(workDir, "file"), groupA.gid)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "change_by_gid",
				Args:    []string{":" + strconv.Itoa(groupB.gid), "file"},
				WorkDir: workDir,
			},
		})
		verifyGroup(t, filepath.Join(workDir, "file"), groupB.gid)
	})

	t.Run("owner_and_group", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "file"), []byte("x"), 0o644)
		setGroup(t, filepath.Join(workDir, "file"), groupA.gid)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "owner_group",
				Args:      []string{cur.Username + ":" + groupB.name, "file"},
				WorkDir:   workDir,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrorVerb},
			},
		})
	})

	t.Run("owner_with_login_group", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "file"), []byte("x"), 0o644)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "login_group",
				Args:      []string{cur.Username + ":", "file"},
				WorkDir:   workDir,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrorVerb},
			},
		})
	})

	t.Run("owner_only", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "file"), []byte("x"), 0o644)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "owner",
				Args:      []string{cur.Username, "file"},
				WorkDir:   workDir,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrorVerb},
			},
		})
	})

	t.Run("multiple_files", func(t *testing.T) {
		workDir := t.TempDir()
		for _, name := range []string{"a", "b", "c"} {
			os.WriteFile(filepath.Join(workDir, name), []byte("x"), 0o644)
			setGroup(t, filepath.Join(workDir, name), groupA.gid)
		}
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "three_files",
				Args:    []string{":" + groupB.name, "a", "b", "c"},
				WorkDir: workDir,
			},
		})
		for _, name := range []string{"a", "b", "c"} {
			verifyGroup(t, filepath.Join(workDir, name), groupB.gid)
		}
	})

	t.Run("reference_file", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "ref"), []byte("r"), 0o644)
		setGroup(t, filepath.Join(workDir, "ref"), groupB.gid)
		os.WriteFile(filepath.Join(workDir, "target"), []byte("t"), 0o644)
		setGroup(t, filepath.Join(workDir, "target"), groupA.gid)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "reference",
				Args:      []string{"--reference=ref", "target"},
				WorkDir:   workDir,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrorVerb},
			},
		})
	})

	t.Run("invalid_user", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "file"), []byte("x"), 0o644)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bad_user",
				Args:      []string{"nonexistent_user_xyz", "file"},
				WorkDir:   workDir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("invalid_group", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "file"), []byte("x"), 0o644)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bad_group",
				Args:      []string{":nonexistent_group_xyz", "file"},
				WorkDir:   workDir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("missing_operand", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no_args",
				Args:      []string{},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("missing_file_operand", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "owner_only",
				Args:      []string{cur.Username},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("nonexistent_file", func(t *testing.T) {
		workDir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no_such_file",
				Args:      []string{":" + groupA.name, "does_not_exist"},
				WorkDir:   workDir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrorVerb},
			},
		})
	})

	t.Run("partial_failure", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "good"), []byte("x"), 0o644)
		setGroup(t, filepath.Join(workDir, "good"), groupA.gid)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "one_good_one_bad",
				Args:      []string{":" + groupB.name, "good", "missing"},
				WorkDir:   workDir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrorVerb},
			},
		})
		verifyGroup(t, filepath.Join(workDir, "good"), groupB.gid)
	})

	t.Run("recursive_directory", func(t *testing.T) {
		workDir := t.TempDir()
		os.MkdirAll(filepath.Join(workDir, "d", "sub"), 0o755)
		os.WriteFile(filepath.Join(workDir, "d", "f1"), []byte("x"), 0o644)
		os.WriteFile(filepath.Join(workDir, "d", "sub", "f2"), []byte("x"), 0o644)
		setGroupRecursive(t, filepath.Join(workDir, "d"), groupA.gid)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "recursive",
				Args:    []string{"-R", ":" + groupB.name, "d"},
				WorkDir: workDir,
			},
		})
		verifyGroup(t, filepath.Join(workDir, "d"), groupB.gid)
		verifyGroup(t, filepath.Join(workDir, "d", "f1"), groupB.gid)
		verifyGroup(t, filepath.Join(workDir, "d", "sub"), groupB.gid)
		verifyGroup(t, filepath.Join(workDir, "d", "sub", "f2"), groupB.gid)
	})

	t.Run("no_dereference_symlink", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "target"), []byte("x"), 0o644)
		os.Symlink("target", filepath.Join(workDir, "link"))
		setGroup(t, filepath.Join(workDir, "target"), groupA.gid)
		setGroup(t, filepath.Join(workDir, "link"), groupA.gid)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "h_flag",
				Args:      []string{"-h", ":" + groupB.name, "link"},
				WorkDir:   workDir,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrorVerb},
			},
		})
		verifyGroup(t, filepath.Join(workDir, "link"), groupB.gid)
		verifyGroup(t, filepath.Join(workDir, "target"), groupA.gid)
	})

	t.Run("recursive_P_mode", func(t *testing.T) {
		workDir := t.TempDir()
		os.MkdirAll(filepath.Join(workDir, "d"), 0o755)
		os.WriteFile(filepath.Join(workDir, "d", "real"), []byte("x"), 0o644)
		os.Symlink("real", filepath.Join(workDir, "d", "sym"))
		setGroupRecursive(t, filepath.Join(workDir, "d"), groupA.gid)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "P_flag",
				Args:      []string{"-R", "-P", ":" + groupB.name, "d"},
				WorkDir:   workDir,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName, normalizeErrorVerb},
			},
		})
		verifyGroup(t, filepath.Join(workDir, "d"), groupB.gid)
		verifyGroup(t, filepath.Join(workDir, "d", "real"), groupB.gid)
		verifyGroup(t, filepath.Join(workDir, "d", "sym"), groupB.gid)
	})
}

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?chown\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("chown"))
}

var errorVerbRe = regexp.MustCompile(
	`(cannot access|changing ownership of)`,
)

func normalizeErrorVerb(b []byte) []byte {
	out := errorVerbRe.ReplaceAll(b, []byte("ERROR_VERB"))
	out = normalizeErrnoCase(out)
	return out
}

var errnoRe = regexp.MustCompile(`(?i)(no such file or directory|permission denied|operation not permitted)`)

func normalizeErrnoCase(b []byte) []byte {
	return errnoRe.ReplaceAllFunc(b, func(m []byte) []byte {
		lower := make([]byte, len(m))
		for i, c := range m {
			if c >= 'A' && c <= 'Z' {
				lower[i] = c + 32
			} else {
				lower[i] = c
			}
		}
		return lower
	})
}

type groupInfo struct {
	name string
	gid  int
}

func testGroups(t *testing.T) []groupInfo {
	t.Helper()
	cur, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current: %v", err)
	}
	gids, err := cur.GroupIds()
	if err != nil {
		t.Fatalf("GroupIds: %v", err)
	}
	var groups []groupInfo
	for _, gidStr := range gids {
		gid, err := strconv.Atoi(gidStr)
		if err != nil {
			continue
		}
		g, err := user.LookupGroupId(gidStr)
		if err != nil {
			continue
		}
		groups = append(groups, groupInfo{name: g.Name, gid: gid})
		if len(groups) == 2 {
			break
		}
	}
	return groups
}

func setGroup(t *testing.T, path string, gid int) {
	t.Helper()
	if err := os.Lchown(path, -1, gid); err != nil {
		t.Fatalf("setGroup(%s, %d): %v", path, gid, err)
	}
}

func setGroupRecursive(t *testing.T, root string, gid int) {
	t.Helper()
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(path, -1, gid)
	})
}

func verifyGroup(t *testing.T, path string, expectedGID int) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat(%s): %v", path, err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatal("Sys() is not *syscall.Stat_t")
	}
	if int(stat.Gid) != expectedGID {
		t.Errorf("group of %s: got %d, want %d", path, stat.Gid, expectedGID)
	}
}

func TestHelp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--help")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--help failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--help produced no output")
	}
}

func TestVersion(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("--version produced no output")
	}
}

func TestReferenceNonexistent(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	cmd := exec.Command(goBin, "--reference=/tmp/no_such_ref_file_xyz", "somefile")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for nonexistent reference file")
	}
	_ = out
	exitCode := cmd.ProcessState.ExitCode()
	if exitCode != 1 {
		t.Errorf("exit code: got %d, want 1", exitCode)
	}
}
