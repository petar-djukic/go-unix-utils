// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ln against gln (GNU coreutils).
//
// Covers prd037-ln R1.1-R1.4, R2.1-R2.4, R3.1-R3.6, R4.1-R4.3.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// discardAll blanks all output so tests check only exit code.
// Used for error messages where the binary name prefix differs.
func discardAll(data []byte) []byte {
	return nil
}

// TestDiff runs differential tests for error cases via RunDiffTests.
// R1.3: hard link to directory fails.
// R1.4: existing destination without -f fails.
// R4.1: non-existent target errors.
// R4.2: permission and cross-device errors.
func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	workDir := t.TempDir()

	// Create a directory for R1.3 test.
	if err := os.Mkdir(filepath.Join(workDir, "somedir"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create files for R1.4 test (existing destination).
	writeFile(t, filepath.Join(workDir, "src.txt"), "source")
	writeFile(t, filepath.Join(workDir, "existing.txt"), "existing")

	// Create a read-only directory for permission denied test (R4.2).
	roDir := filepath.Join(workDir, "readonly")
	if err := os.Mkdir(roDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(workDir, "perm_target.txt"), "data")
	if err := os.Chmod(roDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.Chmod(roDir, 0o755) // best-effort restore for cleanup
	})

	tests := []testutils.DiffTest{
		// No arguments — exit 1.
		{
			Name:      "no_arguments",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.3: hard link to directory — exit 1.
		{
			Name:      "hard_link_directory",
			Args:      []string{"somedir", "newlink"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R1.4: existing destination without -f — exit 1.
		{
			Name:      "existing_destination",
			Args:      []string{"src.txt", "existing.txt"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.1: non-existent target — exit 1.
		{
			Name:      "nonexistent_target",
			Args:      []string{"no_such_file", "newlink"},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.2: permission denied — exit 1.
		{
			Name:      "permission_denied",
			Args:      []string{"perm_target.txt", filepath.Join("readonly", "link")},
			WorkDir:   workDir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.3: --help prints usage and exits 0.
		{
			Name:      "help",
			Args:      []string{"--help"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
		// R4.3: --version prints version info and exits 0.
		{
			Name:      "version",
			Args:      []string{"--version"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardAll},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestHardLinkCreation verifies R1.1: ln TARGET LINK_NAME creates a hard link.
func TestHardLinkCreation(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("single_hard_link", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "hello")
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"target.txt", "link.txt"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"target.txt", "link.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertHardLink(t, goDir, "target.txt", "link.txt")
		assertHardLink(t, refDir, "target.txt", "link.txt")
	})
}

// TestHardLinkIntoDirectory verifies R1.2: ln TARGET... DIRECTORY.
func TestHardLinkIntoDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("multiple_targets_into_dir", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "a.txt"), "aaa")
			writeFile(t, filepath.Join(base, "b.txt"), "bbb")
			if err := os.Mkdir(filepath.Join(base, "dest"), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"a.txt", "b.txt", "dest"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"a.txt", "b.txt", "dest"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertHardLink(t, goDir, "a.txt", filepath.Join("dest", "a.txt"))
		assertHardLink(t, goDir, "b.txt", filepath.Join("dest", "b.txt"))
		assertHardLink(t, refDir, "a.txt", filepath.Join("dest", "a.txt"))
		assertHardLink(t, refDir, "b.txt", filepath.Join("dest", "b.txt"))
	})

	t.Run("single_target_into_dir", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "src.txt"), "data")
			if err := os.Mkdir(filepath.Join(base, "outdir"), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"src.txt", "outdir"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"src.txt", "outdir"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertHardLink(t, goDir, "src.txt", filepath.Join("outdir", "src.txt"))
		assertHardLink(t, refDir, "src.txt", filepath.Join("outdir", "src.txt"))
	})
}

// TestNonExistentTarget verifies R4.1: error on non-existent target.
func TestNonExistentTarget(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	_, _, refExit := execBin(t, refBin, []string{"nonexistent", "newlink"}, refDir)
	_, _, goExit := execBin(t, goBin, []string{"nonexistent", "newlink"}, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	if goExit == 0 {
		t.Error("expected non-zero exit for non-existent target")
	}
}

// TestSymlinkCreation verifies R2.1: -s creates a symbolic link.
func TestSymlinkCreation(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("basic_symlink", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "hello")
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"-s", "target.txt", "link.txt"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"-s", "target.txt", "link.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		// R2.3: target stored as-is.
		assertSymlinkTarget(t, filepath.Join(goDir, "link.txt"), "target.txt")
		assertSymlinkTarget(t, filepath.Join(refDir, "link.txt"), "target.txt")
	})

	t.Run("symlink_long_flag", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "file.txt"), "content")
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"--symbolic", "file.txt", "slink"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"--symbolic", "file.txt", "slink"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertSymlinkTarget(t, filepath.Join(goDir, "slink"), "file.txt")
	})
}

// TestSymlinkToDirectory verifies R2.2: symlinks to directories are allowed.
func TestSymlinkToDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, base := range []string{goDir, refDir} {
		if err := os.Mkdir(filepath.Join(base, "mydir"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	refStdout, _, refExit := execBin(t, refBin, []string{"-s", "mydir", "dirlink"}, refDir)
	goStdout, _, goExit := execBin(t, goBin, []string{"-s", "mydir", "dirlink"}, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
	}
	// R2.2: symlink to directory succeeds.
	assertSymlinkTarget(t, filepath.Join(goDir, "dirlink"), "mydir")
	assertSymlinkTarget(t, filepath.Join(refDir, "dirlink"), "mydir")
}

// TestSymlinkStoresTargetAsIs verifies R2.3: target string stored verbatim.
func TestSymlinkStoresTargetAsIs(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("absolute_target", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		absTarget := "/usr/bin/env"

		_, _, refExit := execBin(t, refBin, []string{"-s", absTarget, "envlink"}, refDir)
		_, _, goExit := execBin(t, goBin, []string{"-s", absTarget, "envlink"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		assertSymlinkTarget(t, filepath.Join(goDir, "envlink"), absTarget)
		assertSymlinkTarget(t, filepath.Join(refDir, "envlink"), absTarget)
	})

	t.Run("relative_target_with_dots", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			if err := os.Mkdir(filepath.Join(base, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(base, "file.txt"), "data")
		}

		_, _, refExit := execBin(t, refBin, []string{"-s", "../file.txt", "sub/link"}, refDir)
		_, _, goExit := execBin(t, goBin, []string{"-s", "../file.txt", "sub/link"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		// R2.3: stores "../file.txt" as-is.
		assertSymlinkTarget(t, filepath.Join(goDir, "sub", "link"), "../file.txt")
		assertSymlinkTarget(t, filepath.Join(refDir, "sub", "link"), "../file.txt")
	})
}

// TestRelativeSymlink verifies R2.4: -r creates relative symlinks.
func TestRelativeSymlink(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("relative_same_dir", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "hello")
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"-sr", "target.txt", "rlink"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"-sr", "target.txt", "rlink"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		// Both should store "target.txt" as relative from same dir.
		goTarget := readSymlink(t, filepath.Join(goDir, "rlink"))
		refTarget := readSymlink(t, filepath.Join(refDir, "rlink"))
		if goTarget != refTarget {
			t.Errorf("symlink target: ref=%q go=%q", refTarget, goTarget)
		}
	})

	t.Run("relative_cross_dir", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "source.txt"), "data")
			if err := os.Mkdir(filepath.Join(base, "subdir"), 0o755); err != nil {
				t.Fatal(err)
			}
		}

		// Create relative symlink from subdir/rlink -> ../source.txt.
		refStdout, _, refExit := execBin(t, refBin, []string{"-s", "-r", "source.txt", "subdir/rlink"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"-s", "-r", "source.txt", "subdir/rlink"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		goTarget := readSymlink(t, filepath.Join(goDir, "subdir", "rlink"))
		refTarget := readSymlink(t, filepath.Join(refDir, "subdir", "rlink"))
		if goTarget != refTarget {
			t.Errorf("symlink target: ref=%q go=%q", refTarget, goTarget)
		}
	})

	t.Run("relative_long_flags", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "file.txt"), "x")
		}

		args := []string{"--symbolic", "--relative", "file.txt", "rlink2"}
		refStdout, _, refExit := execBin(t, refBin, args, refDir)
		goStdout, _, goExit := execBin(t, goBin, args, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		goTarget := readSymlink(t, filepath.Join(goDir, "rlink2"))
		refTarget := readSymlink(t, filepath.Join(refDir, "rlink2"))
		if goTarget != refTarget {
			t.Errorf("symlink target: ref=%q go=%q", refTarget, goTarget)
		}
	})
}

// TestSymlinkIntoDirectory verifies -s with multiple targets into a directory.
func TestSymlinkIntoDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, base := range []string{goDir, refDir} {
		writeFile(t, filepath.Join(base, "a.txt"), "aaa")
		writeFile(t, filepath.Join(base, "b.txt"), "bbb")
		if err := os.Mkdir(filepath.Join(base, "dest"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	args := []string{"-s", "a.txt", "b.txt", "dest"}
	refStdout, _, refExit := execBin(t, refBin, args, refDir)
	goStdout, _, goExit := execBin(t, goBin, args, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
	}
	assertSymlinkTarget(t, filepath.Join(goDir, "dest", "a.txt"), "a.txt")
	assertSymlinkTarget(t, filepath.Join(goDir, "dest", "b.txt"), "b.txt")
	assertSymlinkTarget(t, filepath.Join(refDir, "dest", "a.txt"), "a.txt")
	assertSymlinkTarget(t, filepath.Join(refDir, "dest", "b.txt"), "b.txt")
}

// TestForceOverwrite verifies R3.1: -f removes existing before creating.
func TestForceOverwrite(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("force_hard_link", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "new")
			writeFile(t, filepath.Join(base, "existing.txt"), "old")
		}

		_, _, refExit := execBin(t, refBin, []string{"-f", "target.txt", "existing.txt"}, refDir)
		_, _, goExit := execBin(t, goBin, []string{"-f", "target.txt", "existing.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		assertHardLink(t, goDir, "target.txt", "existing.txt")
	})

	t.Run("force_symlink", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "content")
			writeFile(t, filepath.Join(base, "existing.txt"), "old")
		}

		_, _, refExit := execBin(t, refBin, []string{"-sf", "target.txt", "existing.txt"}, refDir)
		_, _, goExit := execBin(t, goBin, []string{"-sf", "target.txt", "existing.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		assertSymlinkTarget(t, filepath.Join(goDir, "existing.txt"), "target.txt")
	})
}

// TestVerboseOutput verifies R3.4: -v prints link creation to stdout.
func TestVerboseOutput(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("verbose_hard_link", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "hello")
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"-v", "target.txt", "link.txt"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"-v", "target.txt", "link.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertHardLink(t, goDir, "target.txt", "link.txt")
	})

	t.Run("verbose_symlink", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "hello")
		}

		refStdout, _, refExit := execBin(t, refBin, []string{"-sv", "target.txt", "link.txt"}, refDir)
		goStdout, _, goExit := execBin(t, goBin, []string{"-sv", "target.txt", "link.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertSymlinkTarget(t, filepath.Join(goDir, "link.txt"), "target.txt")
	})

	t.Run("verbose_long_flag", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "data")
		}

		args := []string{"--verbose", "-s", "target.txt", "vlink"}
		refStdout, _, refExit := execBin(t, refBin, args, refDir)
		goStdout, _, goExit := execBin(t, goBin, args, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertSymlinkTarget(t, filepath.Join(goDir, "vlink"), "target.txt")
	})

	t.Run("verbose_force_overwrite", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "new")
			writeFile(t, filepath.Join(base, "existing.txt"), "old")
		}

		args := []string{"-sfv", "target.txt", "existing.txt"}
		refStdout, _, refExit := execBin(t, refBin, args, refDir)
		goStdout, _, goExit := execBin(t, goBin, args, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		if !bytes.Equal(refStdout, goStdout) {
			t.Errorf("stdout: ref=%q go=%q", refStdout, goStdout)
		}
		assertSymlinkTarget(t, filepath.Join(goDir, "existing.txt"), "target.txt")
	})
}

// TestBackupSimple verifies R3.5: -b creates a backup with ~ suffix.
func TestBackupSimple(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("backup_default_suffix", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "new")
			writeFile(t, filepath.Join(base, "dest.txt"), "old")
		}

		_, _, refExit := execBin(t, refBin, []string{"-fb", "target.txt", "dest.txt"}, refDir)
		_, _, goExit := execBin(t, goBin, []string{"-fb", "target.txt", "dest.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		// Backup should exist with ~ suffix.
		assertFileExists(t, filepath.Join(goDir, "dest.txt~"))
		assertFileExists(t, filepath.Join(refDir, "dest.txt~"))
		assertHardLink(t, goDir, "target.txt", "dest.txt")
	})

	t.Run("backup_symlink", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "data")
			writeFile(t, filepath.Join(base, "link.txt"), "existing")
		}

		_, _, refExit := execBin(t, refBin, []string{"-sfb", "target.txt", "link.txt"}, refDir)
		_, _, goExit := execBin(t, goBin, []string{"-sfb", "target.txt", "link.txt"}, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		assertFileExists(t, filepath.Join(goDir, "link.txt~"))
		assertSymlinkTarget(t, filepath.Join(goDir, "link.txt"), "target.txt")
	})
}

// TestBackupCustomSuffix verifies R3.6: -S changes the backup suffix.
func TestBackupCustomSuffix(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	t.Run("custom_suffix_short", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "new")
			writeFile(t, filepath.Join(base, "dest.txt"), "old")
		}

		args := []string{"-fb", "-S", ".bak", "target.txt", "dest.txt"}
		_, _, refExit := execBin(t, refBin, args, refDir)
		_, _, goExit := execBin(t, goBin, args, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		assertFileExists(t, filepath.Join(goDir, "dest.txt.bak"))
		assertFileExists(t, filepath.Join(refDir, "dest.txt.bak"))
		assertHardLink(t, goDir, "target.txt", "dest.txt")
	})

	t.Run("custom_suffix_long", func(t *testing.T) {
		t.Parallel()
		goDir := t.TempDir()
		refDir := t.TempDir()

		for _, base := range []string{goDir, refDir} {
			writeFile(t, filepath.Join(base, "target.txt"), "new")
			writeFile(t, filepath.Join(base, "dest.txt"), "old")
		}

		args := []string{"-fb", "--suffix=.orig", "target.txt", "dest.txt"}
		_, _, refExit := execBin(t, refBin, args, refDir)
		_, _, goExit := execBin(t, goBin, args, goDir)

		if refExit != goExit {
			t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
		}
		assertFileExists(t, filepath.Join(goDir, "dest.txt.orig"))
		assertFileExists(t, filepath.Join(refDir, "dest.txt.orig"))
	})
}

// TestBackupNumbered verifies --backup=numbered creates .~N~ backups.
func TestBackupNumbered(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skip("reference binary gln not in PATH")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()

	for _, base := range []string{goDir, refDir} {
		writeFile(t, filepath.Join(base, "target.txt"), "new")
		writeFile(t, filepath.Join(base, "dest.txt"), "old")
	}

	args := []string{"-f", "--backup=numbered", "target.txt", "dest.txt"}
	_, _, refExit := execBin(t, refBin, args, refDir)
	_, _, goExit := execBin(t, goBin, args, goDir)

	if refExit != goExit {
		t.Errorf("exit code: ref=%d go=%d", refExit, goExit)
	}
	assertFileExists(t, filepath.Join(goDir, "dest.txt.~1~"))
	assertFileExists(t, filepath.Join(refDir, "dest.txt.~1~"))
}

// execBin runs a binary and returns stdout, stderr, and exit code.
func execBin(t *testing.T, bin string, args []string, workDir string) ([]byte, []byte, int) {
	t.Helper()

	cmd := exec.Command(bin, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", bin, err)
		}
	}

	return stdout.Bytes(), stderr.Bytes(), exitCode
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// assertHardLink checks that two paths share the same inode (are hard links).
func assertHardLink(t *testing.T, base, file1, file2 string) {
	t.Helper()
	info1, err := os.Stat(filepath.Join(base, file1))
	if err != nil {
		t.Fatalf("stat %s: %v", file1, err)
	}
	info2, err := os.Stat(filepath.Join(base, file2))
	if err != nil {
		t.Fatalf("stat %s: %v", file2, err)
	}
	if !os.SameFile(info1, info2) {
		t.Errorf("%s and %s are not hard links (different inodes)", file1, file2)
	}
}

// assertSymlinkTarget checks that path is a symlink pointing to expected.
func assertSymlinkTarget(t *testing.T, path, expected string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("%s is not a symbolic link", path)
	}
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	if target != expected {
		t.Errorf("symlink %s: got target %q, want %q", path, target, expected)
	}
}

// assertFileExists checks that a file exists at the given path.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("expected file to exist: %s: %v", path, err)
	}
}

// readSymlink returns the target of a symlink.
func readSymlink(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	if err != nil {
		t.Fatalf("readlink %s: %v", path, err)
	}
	return target
}
