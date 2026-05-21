// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd056-cp R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R2.4.
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

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gcp")
	if err != nil {
		t.Skip("reference binary gcp not found")
	}

	env := []string{"LC_ALL=C"}

	binaryNameRe := regexp.MustCompile(`(?m)^(?:/\S+/)?g?cp:`)
	normBin := testutils.NormalizeFunc(func(b []byte) []byte {
		return binaryNameRe.ReplaceAll(b, []byte("cp:"))
	})
	normCase := testutils.NormalizeFunc(func(b []byte) []byte {
		return bytes.ToLower(b)
	})
	normTry := testutils.NormalizeFunc(func(b []byte) []byte {
		var out [][]byte
		for line := range bytes.SplitSeq(b, []byte("\n")) {
			if !bytes.HasPrefix(bytes.TrimSpace(bytes.ToLower(line)), []byte("try ")) {
				out = append(out, bytes.Clone(line))
			}
		}
		return bytes.Join(out, []byte("\n"))
	})
	errNorm := []testutils.NormalizeFunc{normBin, normCase, normTry}
	promptNorm := []testutils.NormalizeFunc{normBin}

	t.Run("r1_1_single_file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "hello world\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "copy",
				Args:    []string{"src.txt", "dest.txt"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"dest.txt": []byte("hello world\n"),
				},
			},
		})
	})

	t.Run("r1_1_into_directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "hello\n")
		os.Mkdir(filepath.Join(dir, "destdir"), 0o755)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "into_dir",
				Args:    []string{"src.txt", "destdir"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"destdir/src.txt": []byte("hello\n"),
				},
			},
		})
	})

	t.Run("r1_1_multi_file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "a.txt", "aaa\n")
		writeFile(t, dir, "b.txt", "bbb\n")
		os.Mkdir(filepath.Join(dir, "destdir"), 0o755)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "multi",
				Args:    []string{"a.txt", "b.txt", "destdir"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"destdir/a.txt": []byte("aaa\n"),
					"destdir/b.txt": []byte("bbb\n"),
				},
			},
		})
	})

	t.Run("r1_1_overwrite_existing", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "new\n")
		writeFile(t, dir, "dest.txt", "old\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "overwrite",
				Args:    []string{"src.txt", "dest.txt"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"dest.txt": []byte("new\n"),
				},
			},
		})
	})

	t.Run("r1_1_error_not_found", func(t *testing.T) {
		dir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "missing_source",
				Args:      []string{"nonexistent.txt", "dest.txt"},
				WorkDir:   dir,
				ExitCode:  1,
				Normalize: errNorm,
				Env:       env,
			},
		})
	})

	t.Run("r1_1_error_source_is_dir", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, "srcdir"), 0o755)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "omitting_dir",
				Args:      []string{"srcdir", "dest.txt"},
				WorkDir:   dir,
				ExitCode:  1,
				Normalize: errNorm,
				Env:       env,
			},
		})
	})

	t.Run("r1_1_error_multi_not_dir", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "a.txt", "a\n")
		writeFile(t, dir, "b.txt", "b\n")
		writeFile(t, dir, "notdir", "x\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "not_dir",
				Args:      []string{"a.txt", "b.txt", "notdir"},
				WorkDir:   dir,
				ExitCode:  1,
				Normalize: errNorm,
				Env:       env,
			},
		})
	})

	t.Run("r1_1_error_no_args", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no_args",
				Args:      []string{},
				ExitCode:  1,
				Normalize: errNorm,
				Env:       env,
			},
		})
	})

	t.Run("r1_1_error_one_arg", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "hello\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "one_arg",
				Args:      []string{"src.txt"},
				WorkDir:   dir,
				ExitCode:  1,
				Normalize: errNorm,
				Env:       env,
			},
		})
	})

	t.Run("r1_2_interactive_yes", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "new\n")
		writeFile(t, dir, "dest.txt", "old\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "yes",
				Args:      []string{"-i", "src.txt", "dest.txt"},
				Stdin:     []byte("y\n"),
				WorkDir:   dir,
				Env:       env,
				Normalize: promptNorm,
				ExpectedFiles: map[string][]byte{
					"dest.txt": []byte("new\n"),
				},
			},
		})
	})

	t.Run("r1_2_interactive_no", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "new\n")
		writeFile(t, dir, "dest.txt", "old\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no",
				Args:      []string{"-i", "src.txt", "dest.txt"},
				Stdin:     []byte("n\n"),
				WorkDir:   dir,
				ExitCode:  1,
				Env:       env,
				Normalize: promptNorm,
				ExpectedFiles: map[string][]byte{
					"dest.txt": []byte("old\n"),
				},
			},
		})
	})

	t.Run("r1_3_force_readonly", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "new\n")
		writeFile(t, dir, "readonly.txt", "old\n")
		os.Chmod(filepath.Join(dir, "readonly.txt"), 0o444)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "force",
				Args:    []string{"-f", "src.txt", "readonly.txt"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"readonly.txt": []byte("new\n"),
				},
			},
		})
	})

	t.Run("r1_4_no_clobber", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "new\n")
		writeFile(t, dir, "existing.txt", "original\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "skip",
				Args:    []string{"-n", "src.txt", "existing.txt"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"existing.txt": []byte("original\n"),
				},
			},
		})
	})

	t.Run("r1_4_no_clobber_new_file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "hello\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "create",
				Args:    []string{"-n", "src.txt", "dest.txt"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"dest.txt": []byte("hello\n"),
				},
			},
		})
	})

	t.Run("r1_4_n_overrides_i", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "src.txt", "new\n")
		writeFile(t, dir, "dest.txt", "original\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "precedence",
				Args:    []string{"-in", "src.txt", "dest.txt"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"dest.txt": []byte("original\n"),
				},
			},
		})
	})

	t.Run("r2_1_recursive", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "src", "sub"), 0o755)
		writeFile(t, dir, "src/a.txt", "aaa\n")
		writeFile(t, dir, "src/sub/b.txt", "bbb\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "nested",
				Args:    []string{"-r", "src", "dest"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"dest/a.txt":     []byte("aaa\n"),
					"dest/sub/b.txt": []byte("bbb\n"),
				},
			},
		})
	})

	t.Run("r2_1_recursive_into_dir", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "src"), 0o755)
		writeFile(t, dir, "src/a.txt", "hello\n")
		os.Mkdir(filepath.Join(dir, "target"), 0o755)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "into_dir",
				Args:    []string{"-r", "src", "target"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"target/src/a.txt": []byte("hello\n"),
				},
			},
		})
	})

	t.Run("r2_1_R_flag", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "src"), 0o755)
		writeFile(t, dir, "src/f.txt", "data\n")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "R_flag",
				Args:    []string{"-R", "src", "dest"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"dest/f.txt": []byte("data\n"),
				},
			},
		})
	})

	t.Run("r2_3_deref", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "src"), 0o755)
		writeFile(t, dir, "src/real.txt", "content\n")
		os.Symlink("real.txt", filepath.Join(dir, "src", "link.txt"))
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "follow_links",
				Args:    []string{"-rL", "src", "dest"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"dest/real.txt": []byte("content\n"),
					"dest/link.txt": []byte("content\n"),
				},
			},
		})
	})

	t.Run("r2_4_preserve_symlink", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "src"), 0o755)
		writeFile(t, dir, "src/real.txt", "hello\n")
		os.Symlink("nonexistent", filepath.Join(dir, "src", "broken"))
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "broken_preserved",
				Args:    []string{"-r", "src", "dest"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"dest/real.txt": []byte("hello\n"),
				},
			},
		})
	})

	t.Run("r2_4_explicit_P", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "src"), 0o755)
		writeFile(t, dir, "src/real.txt", "content\n")
		os.Symlink("real.txt", filepath.Join(dir, "src", "link.txt"))
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:    "explicit_P",
				Args:    []string{"-rP", "src", "dest"},
				WorkDir: dir,
				Env:     env,
				ExpectedFiles: map[string][]byte{
					"dest/real.txt": []byte("content\n"),
				},
			},
		})
	})
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
