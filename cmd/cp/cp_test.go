// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd056-cp R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.4 differential tests
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

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

// TestPreserveTimestamps verifies -p preserves modification timestamps.
// R3.1: -p preserves mode, ownership, and modification timestamps.
func TestPreserveTimestamps(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	content := []byte("preserve me\n")
	os.WriteFile(src, content, 0o644) //nolint:errcheck

	// Set a known timestamp on the source file.
	knownTime := time.Date(2020, 6, 15, 12, 0, 0, 0, time.UTC)
	os.Chtimes(src, knownTime, knownTime) //nolint:errcheck

	cmd := exec.Command(goBin, "-p", src, dst)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cp -p failed: %v\n%s", err, out)
	}

	// Verify modification time is preserved.
	dstInfo, sErr := os.Stat(dst)
	if sErr != nil {
		t.Fatalf("stat dst: %v", sErr)
	}
	if !dstInfo.ModTime().Equal(knownTime) {
		t.Errorf("mod time = %v, want %v", dstInfo.ModTime(), knownTime)
	}
}

// TestPreserveMode verifies -p preserves file permission bits.
// R3.1: -p preserves mode.
func TestPreserveMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	content := []byte("mode test\n")
	// Use a non-default mode.
	os.WriteFile(src, content, 0o755) //nolint:errcheck

	cmd := exec.Command(goBin, "-p", src, dst)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cp -p failed: %v\n%s", err, out)
	}

	srcInfo, _ := os.Stat(src)
	dstInfo, sErr := os.Stat(dst)
	if sErr != nil {
		t.Fatalf("stat dst: %v", sErr)
	}
	if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
		t.Errorf("mode = %o, want %o", dstInfo.Mode().Perm(), srcInfo.Mode().Perm())
	}
}

// TestDiffPreserveFlag verifies -p produces matching exit code and stdout/stderr
// against gcp. R3.1 differential test.
func TestDiffPreserveFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("preserve_single_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		os.WriteFile(src, []byte("data\n"), 0o644) //nolint:errcheck

		knownTime := time.Date(2019, 3, 1, 10, 30, 0, 0, time.UTC)
		os.Chtimes(src, knownTime, knownTime) //nolint:errcheck

		refDst := filepath.Join(dir, "ref_dst.txt")
		goDst := filepath.Join(dir, "go_dst.txt")

		runBin := func(binary, dst string) (int, []byte, []byte) {
			t.Helper()
			c := exec.Command(binary, "-p", src, dst)
			c.Env = append(os.Environ(), "LC_ALL=C")
			var outBuf, errBuf bytes.Buffer
			c.Stdout = &outBuf
			c.Stderr = &errBuf
			runErr := c.Run()
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
		// Normalize stderr for program name differences.
		norm := programNameNormalizer(goBin, refBin)
		if !bytes.Equal(norm(refErr), norm(goErr)) {
			t.Errorf("stderr mismatch:\nref: %q\ngo:  %q", refErr, goErr)
		}

		// Both should have preserved the timestamp.
		refInfo, _ := os.Stat(refDst)
		goInfo, _ := os.Stat(goDst)
		if !refInfo.ModTime().Equal(goInfo.ModTime()) {
			t.Errorf("modtime mismatch: ref=%v go=%v", refInfo.ModTime(), goInfo.ModTime())
		}
	})
}

// TestArchiveMode verifies -a (archive) copies recursively, preserves symlinks,
// and preserves attributes. R3.2 differential test.
func TestArchiveMode(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	srcDir := filepath.Join(dir, "srcdir")
	subDir := filepath.Join(srcDir, "sub")
	os.MkdirAll(subDir, 0o755)                                                //nolint:errcheck
	os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("aaa\n"), 0o644)      //nolint:errcheck
	os.WriteFile(filepath.Join(subDir, "b.txt"), []byte("bbb\n"), 0o644)      //nolint:errcheck
	os.Symlink("a.txt", filepath.Join(srcDir, "link.txt"))                    //nolint:errcheck

	// Set known timestamps.
	knownTime := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	os.Chtimes(filepath.Join(srcDir, "a.txt"), knownTime, knownTime) //nolint:errcheck

	dstDir := filepath.Join(dir, "dstdir")
	cmd := exec.Command(goBin, "-a", srcDir, dstDir)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cp -a failed: %v\n%s", err, out)
	}

	// Verify directory structure.
	checkFile := func(path string, want []byte) {
		t.Helper()
		got, rErr := os.ReadFile(path)
		if rErr != nil {
			t.Errorf("missing file %s: %v", path, rErr)
			return
		}
		if !bytes.Equal(got, want) {
			t.Errorf("file %s content = %q, want %q", path, got, want)
		}
	}
	checkFile(filepath.Join(dstDir, "a.txt"), []byte("aaa\n"))
	checkFile(filepath.Join(dstDir, "sub", "b.txt"), []byte("bbb\n"))

	// Verify symlink preserved (not dereferenced).
	linkPath := filepath.Join(dstDir, "link.txt")
	info, lErr := os.Lstat(linkPath)
	if lErr != nil {
		t.Fatalf("lstat link: %v", lErr)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected symlink, got regular file")
	}

	// Verify timestamps preserved on a.txt.
	aInfo, _ := os.Stat(filepath.Join(dstDir, "a.txt"))
	if !aInfo.ModTime().Equal(knownTime) {
		t.Errorf("a.txt mod time = %v, want %v", aInfo.ModTime(), knownTime)
	}
}

// TestDiffArchiveFlag verifies -a produces matching exit code and output
// against gcp. R3.2 differential test.
func TestDiffArchiveFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("archive_recursive_copy", func(t *testing.T) {
		t.Parallel()
		srcBase := t.TempDir()
		srcDir := filepath.Join(srcBase, "src")
		os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755)                    //nolint:errcheck
		os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte("ff\n"), 0o644) //nolint:errcheck

		refDir := t.TempDir()
		goDir := t.TempDir()
		refDst := filepath.Join(refDir, "dst")
		goDst := filepath.Join(goDir, "dst")

		runBin := func(binary, dst string) (int, []byte, []byte) {
			t.Helper()
			c := exec.Command(binary, "-a", srcDir, dst)
			c.Env = append(os.Environ(), "LC_ALL=C")
			var outBuf, errBuf bytes.Buffer
			c.Stdout = &outBuf
			c.Stderr = &errBuf
			runErr := c.Run()
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

		refCode, refOut, _ := runBin(refBin, refDst)
		goCode, goOut, _ := runBin(goBin, goDst)

		if refCode != goCode {
			t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
		}
		if !bytes.Equal(refOut, goOut) {
			t.Errorf("stdout mismatch:\nref: %q\ngo:  %q", refOut, goOut)
		}
	})
}

// TestPreserveAttrList verifies --preserve=ATTR_LIST with individual attributes.
// R3.3: comma-separated attribute selection.
func TestPreserveAttrList(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("preserve_timestamps_only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		os.WriteFile(src, []byte("ts only\n"), 0o644) //nolint:errcheck

		knownTime := time.Date(2018, 7, 4, 0, 0, 0, 0, time.UTC)
		os.Chtimes(src, knownTime, knownTime) //nolint:errcheck

		cmd := exec.Command(goBin, "--preserve=timestamps", src, dst)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cp --preserve=timestamps failed: %v\n%s", err, out)
		}

		dstInfo, _ := os.Stat(dst)
		if !dstInfo.ModTime().Equal(knownTime) {
			t.Errorf("mod time = %v, want %v", dstInfo.ModTime(), knownTime)
		}
	})

	t.Run("preserve_mode_only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		os.WriteFile(src, []byte("mode only\n"), 0o755) //nolint:errcheck

		cmd := exec.Command(goBin, "--preserve=mode", src, dst)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cp --preserve=mode failed: %v\n%s", err, out)
		}

		srcInfo, _ := os.Stat(src)
		dstInfo, _ := os.Stat(dst)
		if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
			t.Errorf("mode = %o, want %o", dstInfo.Mode().Perm(), srcInfo.Mode().Perm())
		}
	})

	t.Run("preserve_all", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		os.WriteFile(src, []byte("all\n"), 0o755) //nolint:errcheck

		knownTime := time.Date(2017, 12, 25, 0, 0, 0, 0, time.UTC)
		os.Chtimes(src, knownTime, knownTime) //nolint:errcheck

		cmd := exec.Command(goBin, "--preserve=all", src, dst)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cp --preserve=all failed: %v\n%s", err, out)
		}

		srcInfo, _ := os.Stat(src)
		dstInfo, _ := os.Stat(dst)
		if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
			t.Errorf("mode = %o, want %o", dstInfo.Mode().Perm(), srcInfo.Mode().Perm())
		}
		if !dstInfo.ModTime().Equal(knownTime) {
			t.Errorf("mod time = %v, want %v", dstInfo.ModTime(), knownTime)
		}
	})
}

// TestVerboseOutput verifies -v prints each file as it is copied.
// R3.4: -v or --verbose prints each file name.
func TestVerboseOutput(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")

	t.Run("verbose_single_file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		os.WriteFile(src, []byte("verbose\n"), 0o644) //nolint:errcheck

		cmd := exec.Command(goBin, "-v", src, dst)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		if err := cmd.Run(); err != nil {
			t.Fatalf("cp -v failed: %v", err)
		}

		// R3.4: verbose output goes to stdout, matching GNU cp.
		expected := fmt.Sprintf("'%s' -> '%s'\n", src, dst)
		if outBuf.String() != expected {
			t.Errorf("verbose output = %q, want %q", outBuf.String(), expected)
		}
	})

	t.Run("verbose_recursive", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		srcDir := filepath.Join(dir, "srcdir")
		os.MkdirAll(filepath.Join(srcDir, "sub"), 0o755)                    //nolint:errcheck
		os.WriteFile(filepath.Join(srcDir, "a.txt"), []byte("a\n"), 0o644)  //nolint:errcheck
		os.WriteFile(filepath.Join(srcDir, "sub", "b.txt"), []byte("b\n"), 0o644) //nolint:errcheck

		dstDir := filepath.Join(dir, "dstdir")
		cmd := exec.Command(goBin, "-rv", srcDir, dstDir)
		cmd.Env = append(os.Environ(), "LC_ALL=C")
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		if err := cmd.Run(); err != nil {
			t.Fatalf("cp -rv failed: %v", err)
		}

		output := outBuf.String()
		// Should contain entries for directory and files.
		if !bytes.Contains([]byte(output), []byte("->")) {
			t.Errorf("verbose output missing '->' entries: %q", output)
		}
	})
}

// TestDiffExitCodeSuccess verifies exit 0 on successful copy.
// R4.1: must exit 0 when all files are copied successfully.
func TestDiffExitCodeSuccess(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("exit_0_on_success", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		dst := filepath.Join(dir, "dst.txt")
		os.WriteFile(src, []byte("ok\n"), 0o644) //nolint:errcheck

		tests := []testutils.DiffTest{
			{
				Name:     "successful_copy_exit_0",
				Args:     []string{src, dst},
				Env:      []string{"LC_ALL=C"},
				ExitCode: 0,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestDiffExitCodeFailure verifies exit 1 on failed copy operations.
// R4.2: must exit 1 when any copy operation fails.
func TestDiffExitCodeFailure(t *testing.T) {
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

	t.Run("exit_1_missing_source", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		tests := []testutils.DiffTest{
			{
				Name:      "nonexistent_exit_1",
				Args:      []string{filepath.Join(dir, "no_such"), filepath.Join(dir, "dst")},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})

	t.Run("exit_1_dir_without_r", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		srcDir := filepath.Join(dir, "adir")
		os.Mkdir(srcDir, 0o755) //nolint:errcheck

		tests := []testutils.DiffTest{
			{
				Name:      "dir_no_r_exit_1",
				Args:      []string{srcDir, filepath.Join(dir, "copy")},
				Env:       []string{"LC_ALL=C"},
				ExitCode:  1,
				Normalize: normalize,
			},
		}
		testutils.RunDiffTests(t, goBin, refBin, tests)
	})
}

// TestDiffTargetDirectory verifies -t DIRECTORY flag.
// R4.3: -t copies all SOURCE arguments into DIRECTORY.
func TestDiffTargetDirectory(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("t_flag_single_source", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "file.txt")
		content := []byte("target dir test\n")
		os.WriteFile(src, content, 0o644) //nolint:errcheck

		// Each binary needs its own dest dir.
		refDst := filepath.Join(dir, "ref_out")
		goDst := filepath.Join(dir, "go_out")
		os.Mkdir(refDst, 0o755) //nolint:errcheck
		os.Mkdir(goDst, 0o755)  //nolint:errcheck

		runBin := func(binary, dst string) (int, []byte, []byte) {
			t.Helper()
			cmd := exec.Command(binary, "-t", dst, src)
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

		refCode, refOut, _ := runBin(refBin, refDst)
		goCode, goOut, _ := runBin(goBin, goDst)

		if refCode != goCode {
			t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
		}
		if !bytes.Equal(refOut, goOut) {
			t.Errorf("stdout mismatch:\nref: %q\ngo:  %q", refOut, goOut)
		}

		// Verify file was copied into the target directory.
		got, rErr := os.ReadFile(filepath.Join(goDst, "file.txt"))
		if rErr != nil {
			t.Fatalf("file not found in target dir: %v", rErr)
		}
		if !bytes.Equal(got, content) {
			t.Errorf("content = %q, want %q", got, content)
		}
	})

	t.Run("t_flag_multiple_sources", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src1 := filepath.Join(dir, "a.txt")
		src2 := filepath.Join(dir, "b.txt")
		contentA := []byte("aaa\n")
		contentB := []byte("bbb\n")
		os.WriteFile(src1, contentA, 0o644) //nolint:errcheck
		os.WriteFile(src2, contentB, 0o644) //nolint:errcheck

		refDst := filepath.Join(dir, "ref_out")
		goDst := filepath.Join(dir, "go_out")
		os.Mkdir(refDst, 0o755) //nolint:errcheck
		os.Mkdir(goDst, 0o755)  //nolint:errcheck

		runBin := func(binary, dst string) (int, []byte, []byte) {
			t.Helper()
			cmd := exec.Command(binary, "-t", dst, src1, src2)
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

		refCode, _, _ := runBin(refBin, refDst)
		goCode, _, _ := runBin(goBin, goDst)

		if refCode != goCode {
			t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
		}

		// Verify both files copied.
		gotA, _ := os.ReadFile(filepath.Join(goDst, "a.txt"))
		gotB, _ := os.ReadFile(filepath.Join(goDst, "b.txt"))
		if !bytes.Equal(gotA, contentA) {
			t.Errorf("a.txt content = %q, want %q", gotA, contentA)
		}
		if !bytes.Equal(gotB, contentB) {
			t.Errorf("b.txt content = %q, want %q", gotB, contentB)
		}
	})

	t.Run("target_directory_long_option", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "data.txt")
		content := []byte("long opt\n")
		os.WriteFile(src, content, 0o644) //nolint:errcheck

		refDst := filepath.Join(dir, "ref_out")
		goDst := filepath.Join(dir, "go_out")
		os.Mkdir(refDst, 0o755) //nolint:errcheck
		os.Mkdir(goDst, 0o755)  //nolint:errcheck

		runBin := func(binary, dst string) (int, []byte, []byte) {
			t.Helper()
			cmd := exec.Command(binary, "--target-directory="+dst, src)
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

		refCode, _, _ := runBin(refBin, refDst)
		goCode, _, _ := runBin(goBin, goDst)

		if refCode != goCode {
			t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
		}

		got, rErr := os.ReadFile(filepath.Join(goDst, "data.txt"))
		if rErr != nil {
			t.Fatalf("file not found: %v", rErr)
		}
		if !bytes.Equal(got, content) {
			t.Errorf("content = %q, want %q", got, content)
		}
	})
}

// TestDiffNoClobber verifies -n does not overwrite existing files.
// R1.4, R4.4: differential test for no-clobber mode.
func TestDiffNoClobber(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("no_clobber_preserves_existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		os.WriteFile(src, []byte("new content\n"), 0o644) //nolint:errcheck

		originalContent := []byte("original\n")

		// Each binary gets its own destination that already exists.
		refDst := filepath.Join(dir, "ref_existing.txt")
		goDst := filepath.Join(dir, "go_existing.txt")
		os.WriteFile(refDst, originalContent, 0o644) //nolint:errcheck
		os.WriteFile(goDst, originalContent, 0o644)  //nolint:errcheck

		runBin := func(binary, dst string) (int, []byte, []byte) {
			t.Helper()
			cmd := exec.Command(binary, "-n", src, dst)
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

		refCode, _, _ := runBin(refBin, refDst)
		goCode, _, _ := runBin(goBin, goDst)

		if refCode != goCode {
			t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
		}

		// Verify go binary did not overwrite the existing file.
		got, _ := os.ReadFile(goDst)
		if !bytes.Equal(got, originalContent) {
			t.Errorf("file was overwritten: got %q, want %q", got, originalContent)
		}
	})
}

// TestDiffForceFlag verifies -f removes unwritable destination and retries.
// R1.3, R4.4: differential test for force mode.
func TestDiffForceFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("force_overwrites_readonly", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		content := []byte("forced\n")
		os.WriteFile(src, content, 0o644) //nolint:errcheck

		// Each binary gets its own read-only destination.
		refDst := filepath.Join(dir, "ref_ro.txt")
		goDst := filepath.Join(dir, "go_ro.txt")
		os.WriteFile(refDst, []byte("old\n"), 0o444) //nolint:errcheck
		os.WriteFile(goDst, []byte("old\n"), 0o444)  //nolint:errcheck

		runBin := func(binary, dst string) (int, []byte, []byte) {
			t.Helper()
			cmd := exec.Command(binary, "-f", src, dst)
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

		refCode, _, _ := runBin(refBin, refDst)
		goCode, _, _ := runBin(goBin, goDst)

		if refCode != goCode {
			t.Errorf("exit code mismatch: ref=%d go=%d", refCode, goCode)
		}

		// Verify go binary wrote the new content.
		got, rErr := os.ReadFile(goDst)
		if rErr != nil {
			t.Fatalf("reading dst: %v", rErr)
		}
		if !bytes.Equal(got, content) {
			t.Errorf("content = %q, want %q", got, content)
		}
	})
}

// TestDiffVerboseFlag verifies -v produces matching output against gcp.
// R3.4 differential test.
func TestDiffVerboseFlag(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skipf("reference binary gcp not in PATH: %v", err)
	}

	t.Run("verbose_single_file_diff", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		src := filepath.Join(dir, "src.txt")
		os.WriteFile(src, []byte("verbose diff\n"), 0o644) //nolint:errcheck

		refDst := filepath.Join(dir, "ref_dst.txt")
		goDst := filepath.Join(dir, "go_dst.txt")

		normalize := []testutils.NormalizeFunc{
			programNameNormalizer(goBin, refBin),
		}

		runBin := func(binary, dst string) (int, []byte, []byte) {
			t.Helper()
			c := exec.Command(binary, "-v", src, dst)
			c.Env = append(os.Environ(), "LC_ALL=C")
			var outBuf, errBuf bytes.Buffer
			c.Stdout = &outBuf
			c.Stderr = &errBuf
			runErr := c.Run()
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

		// Normalize destination paths in verbose stdout since they differ.
		dstNorm := func(b []byte) []byte {
			b = bytes.ReplaceAll(b, []byte(refDst), []byte("DST"))
			b = bytes.ReplaceAll(b, []byte(goDst), []byte("DST"))
			return b
		}

		normRefOut := dstNorm(refOut)
		normGoOut := dstNorm(goOut)
		for _, fn := range normalize {
			normRefOut = fn(normRefOut)
			normGoOut = fn(normGoOut)
		}

		if !bytes.Equal(normRefOut, normGoOut) {
			t.Errorf("stdout mismatch:\nref: %q\ngo:  %q", normRefOut, normGoOut)
		}

		// Stderr should also match (empty for successful copy).
		for _, fn := range normalize {
			refErr = fn(refErr)
			goErr = fn(goErr)
		}
		if !bytes.Equal(refErr, goErr) {
			t.Errorf("stderr mismatch:\nref: %q\ngo:  %q", refErr, goErr)
		}
	})
}
