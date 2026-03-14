// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd056-cp R1.1, R1.2, R1.3, R1.4 differential tests
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
