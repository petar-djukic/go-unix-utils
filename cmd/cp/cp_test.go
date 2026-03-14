// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd056-cp R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4 differential tests
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// programNameNormalizer replaces the binary name (gcp or the full Go binary
// path) with the canonical name "cp" so stderr messages are comparable.
func programNameNormalizer(goBin, refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte("cp"))
		b = bytes.ReplaceAll(b, []byte(goBin), []byte("cp"))
		b = bytes.ReplaceAll(b, []byte("gcp"), []byte("cp"))
		return b
	}
}

// tryHelpNormalizer removes the "Try 'cp --help'..." line that GNU cp appends
// to some error messages. The Go implementation does not emit this line.
var tryHelpRe = regexp.MustCompile(`(?m)^Try '.*' for more information\.\n`)

var tryHelpNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	return tryHelpRe.ReplaceAll(b, nil)
}

// TestDiffSingleFileCopy verifies single-file copy produces identical content.
// R1.1, R1.2: copy SOURCE to DEST with byte-for-byte content match.
func TestDiffSingleFileCopy(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("copy_regular_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		content := []byte("hello world\n")
		if wErr := os.WriteFile(src, content, 0o644); wErr != nil {
			t.Fatalf("setup: %v", wErr)
		}
		tests := []testutils.DiffTest{
			{
				Name:          "single_file_copy",
				Args:          []string{src, dst},
				Env:           []string{"LC_ALL=C"},
				ExitCode:      0,
				ExpectedFiles: map[string][]byte{filepath.Base(dst): content},
				WorkDir:       dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestDiffCopyIntoDirectory verifies copying a file into an existing directory.
// R1.4: destination is a directory; source basename is used for the new filename.
func TestDiffCopyIntoDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("copy_into_directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "source.txt")
		destDir := filepath.Join(dir, "destdir")
		content := []byte("copy into dir\n")
		if wErr := os.WriteFile(src, content, 0o644); wErr != nil {
			t.Fatalf("setup: %v", wErr)
		}
		if mkErr := os.Mkdir(destDir, 0o755); mkErr != nil {
			t.Fatalf("setup: %v", mkErr)
		}
		tests := []testutils.DiffTest{
			{
				Name:          "into_existing_directory",
				Args:          []string{src, destDir},
				Env:           []string{"LC_ALL=C"},
				ExitCode:      0,
				ExpectedFiles: map[string][]byte{filepath.Join("destdir", "source.txt"): content},
				WorkDir:       dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestDiffErrors verifies error cases produce matching exit codes and stderr.
// R1.3: missing source, source equals destination, directory without -r.
func TestDiffErrors(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		tryHelpNormalizer,
	}

	t.Run("missing_source", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		tests := []testutils.DiffTest{
			{
				Name:      "nonexistent_source",
				Args:      []string{filepath.Join(dir, "no_such_file"), filepath.Join(dir, "dst.txt")},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("omitting_directory", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		subdir := filepath.Join(dir, "subdir")
		if mkErr := os.Mkdir(subdir, 0o755); mkErr != nil {
			t.Fatalf("setup: %v", mkErr)
		}
		tests := []testutils.DiffTest{
			{
				Name:      "directory_without_r",
				Args:      []string{subdir, filepath.Join(dir, "copy")},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("missing_operand", func(t *testing.T) {
		t.Parallel()
		tests := []testutils.DiffTest{
			{
				Name:      "no_arguments",
				Args:      []string{},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestDiffMultipleSourcesCopy verifies copying multiple sources into a directory.
// R1.1: multiple SOURCE arguments copied into DEST directory.
func TestDiffMultipleSourcesCopy(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("multi_source_into_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src1 := filepath.Join(dir, "a.txt")
		src2 := filepath.Join(dir, "b.txt")
		destDir := filepath.Join(dir, "out")
		contentA := []byte("aaa\n")
		contentB := []byte("bbb\n")
		os.WriteFile(src1, contentA, 0o644) //nolint:errcheck
		os.WriteFile(src2, contentB, 0o644) //nolint:errcheck
		if mkErr := os.Mkdir(destDir, 0o755); mkErr != nil {
			t.Fatalf("setup: %v", mkErr)
		}
		tests := []testutils.DiffTest{
			{
				Name: "two_files_into_directory",
				Args: []string{src1, src2, destDir},
				Env:  []string{"LC_ALL=C"},
				ExpectedFiles: map[string][]byte{
					filepath.Join("out", "a.txt"): contentA,
					filepath.Join("out", "b.txt"): contentB,
				},
				ExitCode: 0,
				WorkDir:  dir,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestDiffRecursiveCopy verifies recursive directory copying with -r.
// R2.1: -r copies directories recursively preserving structure.
// Note: differential tests compare exit code and stdout/stderr only. Filesystem
// state is verified in TestRecursiveCopyState which runs the go binary alone.
func TestDiffRecursiveCopy(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	// Each binary needs its own temp dir so the first run doesn't affect the
	// second. We run each independently and compare outputs manually.
	for _, flagVariant := range []struct {
		name string
		flag string
	}{
		{"r_flag", "-r"},
		{"R_flag", "-R"},
		{"recursive_long", "--recursive"},
	} {
		fv := flagVariant
		t.Run(fv.name, func(t *testing.T) {
			t.Parallel()

			// Shared source tree.
			srcBase := t.TempDir()
			srcDir := filepath.Join(srcBase, "srcdir")
			subDir := filepath.Join(srcDir, "sub")
			os.MkdirAll(subDir, 0o755)                                       //nolint:errcheck
			os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("aaa\n"), 0o644) //nolint:errcheck
			os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("bbb\n"), 0o644) //nolint:errcheck

			// Separate dest dirs for ref and go binaries.
			refDir := t.TempDir()
			goDir := t.TempDir()

			refDst := filepath.Join(refDir, "dstdir")
			goDst := filepath.Join(goDir, "dstdir")

			refTests := []testutils.DiffTest{{
				Name:     fv.name + "_ref_vs_go",
				Args:     []string{fv.flag, srcDir, refDst},
				Env:      []string{"LC_ALL=C"},
				ExitCode: 0,
			}}
			goTests := []testutils.DiffTest{{
				Name:     fv.name + "_ref_vs_go",
				Args:     []string{fv.flag, srcDir, goDst},
				Env:      []string{"LC_ALL=C"},
				ExitCode: 0,
			}}

			// Run ref binary with refDst, go binary with goDst via
			// separate RunDiffTests calls using goBin as both ref and go
			// in each call, then compare results. Actually, just run each
			// binary directly and compare exit codes.
			runAndCheck := func(binary, dst string) (int, []byte, []byte) {
				t.Helper()
				cmd := exec.Command(binary, fv.flag, srcDir, dst)
				cmd.Env = append(os.Environ(), "LC_ALL=C")
				var outBuf, errBuf bytes.Buffer
				cmd.Stdout = &outBuf
				cmd.Stderr = &errBuf
				runErr := cmd.Run()
				code := 0
				if runErr != nil {
					if exitErr, ok := runErr.(*exec.ExitError); ok {
						code = exitErr.ExitCode()
					} else {
						t.Fatalf("failed to run %q: %v", binary, runErr)
					}
				}
				return code, outBuf.Bytes(), errBuf.Bytes()
			}

			refCode, refOut, refErr := runAndCheck(refBin, refDst)
			goCode, goOut, goErr := runAndCheck(goBin, goDst)

			_ = refTests
			_ = goTests

			if refCode != goCode {
				t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
			}
			if !bytes.Equal(refOut, goOut) {
				t.Errorf("stdout mismatch:\nref: %q\ngo:  %q", refOut, goOut)
			}
			if !bytes.Equal(refErr, goErr) {
				t.Errorf("stderr mismatch:\nref: %q\ngo:  %q", refErr, goErr)
			}
		})
	}
}

// TestRecursiveCopyState verifies the filesystem state after recursive copy
// using only the go binary. This avoids the shared-directory problem in
// differential tests where both binaries write to the same destination.
// R2.1: directory structure is preserved.
func TestRecursiveCopyState(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "srcdir")
	subDir := filepath.Join(srcDir, "sub")
	os.MkdirAll(subDir, 0o755)                                            //nolint:errcheck
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("file a\n"), 0o644) //nolint:errcheck
	os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("file b\n"), 0o644) //nolint:errcheck

	dstDir := filepath.Join(dir, "dstdir")
	cmd := exec.Command(goBin, "-r", srcDir, dstDir)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cp -r failed: %v\n%s", err, out)
	}

	// Verify directory structure preserved.
	checkFile := func(path string, want []byte) {
		t.Helper()
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("missing file %s: %v", path, err)
			return
		}
		if !bytes.Equal(got, want) {
			t.Errorf("file %s content = %q, want %q", path, got, want)
		}
	}
	checkFile(filepath.Join(dstDir, "a.txt"), []byte("file a\n"))
	checkFile(filepath.Join(dstDir, "sub", "b.txt"), []byte("file b\n"))
}

// TestDiffDirectoryWithoutR verifies that copying a directory without -r fails.
// R2.2: must refuse and print error to stderr.
func TestDiffDirectoryWithoutR(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	normalize := []testutils.NormalizeFunc{
		programNameNormalizer(goBin, refBin),
		tryHelpNormalizer,
	}

	t.Run("dir_without_r", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		srcDir := filepath.Join(dir, "mydir")
		os.Mkdir(srcDir, 0o755) //nolint:errcheck

		tests := []testutils.DiffTest{
			{
				Name:      "omit_directory_no_r",
				Args:      []string{srcDir, filepath.Join(dir, "copy")},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestDiffSymlinkDereference verifies -L follows symlinks in source.
// R2.3: -L copies the file the symlink points to, not the link itself.
func TestDiffSymlinkDereference(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("dereference_symlink_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		realFile := filepath.Join(dir, "real.txt")
		content := []byte("real content\n")
		os.WriteFile(realFile, content, 0o644) //nolint:errcheck
		linkFile := filepath.Join(dir, "link.txt")
		os.Symlink(realFile, linkFile) //nolint:errcheck

		// Use separate destinations: ref creates copy_ref.txt, go creates copy_go.txt.
		// Both should succeed with exit 0 and no output.
		refDst := filepath.Join(dir, "copy_ref.txt")
		goDst := filepath.Join(dir, "copy_go.txt")

		runBin := func(binary, dst string) (int, []byte, []byte) {
			t.Helper()
			cmd := exec.Command(binary, "-L", linkFile, dst)
			cmd.Env = append(os.Environ(), "LC_ALL=C")
			var outBuf, errBuf bytes.Buffer
			cmd.Stdout = &outBuf
			cmd.Stderr = &errBuf
			runErr := cmd.Run()
			code := 0
			if runErr != nil {
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					code = exitErr.ExitCode()
				} else {
					t.Fatalf("failed to run %q: %v", binary, runErr)
				}
			}
			return code, outBuf.Bytes(), errBuf.Bytes()
		}

		refCode, refOut, refErr := runBin(refBin, refDst)
		goCode, goOut, goErr := runBin(goBin, goDst)

		if refCode != goCode {
			t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
		}
		if !bytes.Equal(refOut, goOut) {
			t.Errorf("stdout mismatch:\nref: %q\ngo:  %q", refOut, goOut)
		}
		if !bytes.Equal(refErr, goErr) {
			t.Errorf("stderr mismatch:\nref: %q\ngo:  %q", refErr, goErr)
		}

		// Verify go binary produced a regular file, not a symlink.
		info, sErr := os.Lstat(goDst)
		if sErr != nil {
			t.Fatalf("lstat copy: %v", sErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("expected regular file, got symlink")
		}
		// Verify content matches.
		got, _ := os.ReadFile(goDst)
		if !bytes.Equal(got, content) {
			t.Errorf("copy content = %q, want %q", got, content)
		}
	})
}

// TestSymlinkNoDereferenceState verifies -P and default -r symlink handling
// using only the go binary. R2.4: symlinks are preserved.
func TestSymlinkNoDereferenceState(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("r_preserves_symlink_by_default", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		srcDir := filepath.Join(dir, "srcdir")
		os.Mkdir(srcDir, 0o755)                                              //nolint:errcheck
		os.WriteFile(filepath.Join(srcDir, "real.txt"), []byte("real\n"), 0o644) //nolint:errcheck
		os.Symlink("real.txt", filepath.Join(srcDir, "link.txt"))            //nolint:errcheck

		dstDir := filepath.Join(dir, "dstdir")
		cmd := exec.Command(goBin, "-r", srcDir, dstDir)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cp -r failed: %v\n%s", err, out)
		}

		// Verify symlink preserved.
		linkPath := filepath.Join(dstDir, "link.txt")
		info, lErr := os.Lstat(linkPath)
		if lErr != nil {
			t.Fatalf("lstat: %v", lErr)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected symlink, got regular file")
		}
		target, rErr := os.Readlink(linkPath)
		if rErr != nil {
			t.Fatalf("readlink: %v", rErr)
		}
		if target != "real.txt" {
			t.Errorf("symlink target = %q, want %q", target, "real.txt")
		}
	})

	t.Run("explicit_P_preserves_symlink", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		srcDir := filepath.Join(dir, "srcdir")
		os.Mkdir(srcDir, 0o755)                                                //nolint:errcheck
		os.WriteFile(filepath.Join(srcDir, "data.txt"), []byte("hello\n"), 0o644) //nolint:errcheck
		os.Symlink("data.txt", filepath.Join(srcDir, "sym.txt"))               //nolint:errcheck

		dstDir := filepath.Join(dir, "dstdir")
		cmd := exec.Command(goBin, "-rP", srcDir, dstDir)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cp -rP failed: %v\n%s", err, out)
		}

		linkPath := filepath.Join(dstDir, "sym.txt")
		info, lErr := os.Lstat(linkPath)
		if lErr != nil {
			t.Fatalf("lstat: %v", lErr)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("expected symlink, got regular file")
		}
	})

	t.Run("rL_dereferences_symlink_in_dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		srcDir := filepath.Join(dir, "srcdir")
		os.Mkdir(srcDir, 0o755)                                             //nolint:errcheck
		realFile := filepath.Join(dir, "target.txt")
		content := []byte("target data\n")
		os.WriteFile(realFile, content, 0o644)                              //nolint:errcheck
		os.Symlink(realFile, filepath.Join(srcDir, "link.txt"))             //nolint:errcheck

		dstDir := filepath.Join(dir, "dstdir")
		cmd := exec.Command(goBin, "-rL", srcDir, dstDir)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cp -rL failed: %v\n%s", err, out)
		}

		// Verify it's a regular file, not a symlink.
		linkCopy := filepath.Join(dstDir, "link.txt")
		info, lErr := os.Lstat(linkCopy)
		if lErr != nil {
			t.Fatalf("lstat: %v", lErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("expected regular file, got symlink")
		}
		got, _ := os.ReadFile(linkCopy)
		if !bytes.Equal(got, content) {
			t.Errorf("file content = %q, want %q", got, content)
		}
	})
}
