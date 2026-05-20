// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd036-mktemp R4.1, R4.2, R4.3, R4.4.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skip("reference binary gmktemp not found")
	}

	t.Run("default_no_args", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "default",
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10)},
			},
		})
		verifyCreated(t, tmpdir, "tmp.", 10, 0o600, false)
	})

	t.Run("custom_template", func(t *testing.T) {
		tmpdir := t.TempDir()
		template := filepath.Join(tmpdir, "myfileXXXXXX")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "custom",
				Args:      []string{template},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("myfile", 6)},
			},
		})
		verifyCreated(t, tmpdir, "myfile", 6, 0o600, false)
	})

	t.Run("custom_template_min_xs", func(t *testing.T) {
		tmpdir := t.TempDir()
		template := filepath.Join(tmpdir, "fooXXX")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "min_xs",
				Args:      []string{template},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("foo", 3)},
			},
		})
		verifyCreated(t, tmpdir, "foo", 3, 0o600, false)
	})

	t.Run("directory_short_flag", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "d_flag",
				Args:      []string{"-d"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10)},
			},
		})
		verifyCreated(t, tmpdir, "tmp.", 10, 0o700, true)
	})

	t.Run("directory_long_flag", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "directory_flag",
				Args:      []string{"--directory"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10)},
			},
		})
		verifyCreated(t, tmpdir, "tmp.", 10, 0o700, true)
	})

	t.Run("directory_custom_template", func(t *testing.T) {
		tmpdir := t.TempDir()
		template := filepath.Join(tmpdir, "myXXXXXX")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "d_custom",
				Args:      []string{"-d", template},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("my", 6)},
			},
		})
		verifyCreated(t, tmpdir, "my", 6, 0o700, true)
	})

	t.Run("directory_p_flag", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "d_p",
				Args:      []string{"-d", "-p", tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10)},
			},
		})
		verifyCreated(t, tmpdir, "tmp.", 10, 0o700, true)
	})

	t.Run("directory_tmpdir_eq", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "d_tmpdir_eq",
				Args:      []string{"-d", "--tmpdir=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10)},
			},
		})
		verifyCreated(t, tmpdir, "tmp.", 10, 0o700, true)
	})

	t.Run("directory_tmpdir_no_value", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "d_tmpdir_nv",
				Args:      []string{"-d", "--tmpdir"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10)},
			},
		})
		verifyCreated(t, tmpdir, "tmp.", 10, 0o700, true)
	})

	t.Run("p_flag_explicit_dir", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "p_dir",
				Args:      []string{"-p", tmpdir, "myXXXXXX"},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("my", 6)},
			},
		})
		verifyCreated(t, tmpdir, "my", 6, 0o600, false)
	})

	t.Run("tmpdir_equals_dir", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "tmpdir_eq",
				Args:      []string{"--tmpdir=" + tmpdir, "myXXXXXX"},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("my", 6)},
			},
		})
		verifyCreated(t, tmpdir, "my", 6, 0o600, false)
	})

	t.Run("tmpdir_no_value", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "tmpdir_no_val",
				Args:      []string{"--tmpdir", "myXXXXXX"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("my", 6)},
			},
		})
		verifyCreated(t, tmpdir, "my", 6, 0o600, false)
	})

	t.Run("suffix", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "suffix_txt",
				Args:      []string{"--suffix=.txt"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10)},
			},
		})
		verifySuffix(t, tmpdir, ".txt")
	})

	t.Run("suffix_with_dir", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "suffix_p",
				Args:      []string{"-d", "-p", tmpdir, "--suffix=.d", "testXXXXXX"},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("test", 6)},
			},
		})
		verifySuffix(t, tmpdir, ".d")
		verifyCreatedWithSuffix(t, tmpdir, "test", 6, ".d", 0o700, true)
	})

	t.Run("t_legacy_mode", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "t_flag",
				Args:      []string{"-t", "myXXXXXX"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("my", 6)},
			},
		})
		verifyCreated(t, tmpdir, "my", 6, 0o600, false)
	})

	t.Run("error_suffix_slash", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "suffix_slash",
				Args:      []string{"--suffix=/bad"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("error_too_few_xs", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "two_xs",
				Args:      []string{"fooXX"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
			{
				Name:      "one_x",
				Args:      []string{"fooX"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
			{
				Name:      "no_xs",
				Args:      []string{"foo"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("error_too_many_templates", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "two_templates",
				Args:      []string{"fooXXX", "barXXX"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("dry_run_short_flag", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "u_flag",
				Args:      []string{"-u"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10), normalizeDryRunWarning},
			},
		})
		verifyNotCreated(t, tmpdir)
	})

	t.Run("dry_run_long_flag", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "dry_run",
				Args:      []string{"--dry-run"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10), normalizeDryRunWarning},
			},
		})
		verifyNotCreated(t, tmpdir)
	})

	t.Run("dry_run_directory", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "u_d",
				Args:      []string{"-u", "-d"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10), normalizeDryRunWarning},
			},
		})
		verifyNotCreated(t, tmpdir)
	})

	t.Run("quiet_creation_error", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "q_flag",
				Args:     []string{"-q", "-p", "/nonexistent_mktemp_test_dir", "testXXXXXX"},
				ExitCode: 1,
			},
		})
	})

	t.Run("quiet_long_creation_error", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "quiet_flag",
				Args:     []string{"--quiet", "-p", "/nonexistent_mktemp_test_dir", "testXXXXXX"},
				ExitCode: 1,
			},
		})
	})

	t.Run("quiet_validation_error", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "q_validation",
				Args:      []string{"-q", "fooXX"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("t_legacy_directory", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "t_d_flag",
				Args:      []string{"-t", "-d", "myXXXXXX"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("my", 6)},
			},
		})
		verifyCreated(t, tmpdir, "my", 6, 0o700, true)
	})

	t.Run("dry_run_custom_template", func(t *testing.T) {
		tmpdir := t.TempDir()
		template := filepath.Join(tmpdir, "testXXXXXX")
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "u_custom",
				Args:      []string{"-u", template},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("test", 6), normalizeDryRunWarning},
			},
		})
		verifyNotCreated(t, tmpdir)
	})

	t.Run("dry_run_with_suffix", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "u_suffix",
				Args:      []string{"-u", "--suffix=.txt"},
				Env:       []string{"TMPDIR=" + tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10), normalizeDryRunWarning},
			},
		})
		verifyNotCreated(t, tmpdir)
	})

	t.Run("p_flag_default_template", func(t *testing.T) {
		tmpdir := t.TempDir()
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "p_default",
				Args:      []string{"-p", tmpdir},
				Normalize: []testutils.NormalizeFunc{makeTemplateNormalizer("tmp.", 10)},
			},
		})
		verifyCreated(t, tmpdir, "tmp.", 10, 0o600, false)
	})
}

func makeTemplateNormalizer(prefix string, xCount int) testutils.NormalizeFunc {
	pattern := regexp.MustCompile(
		regexp.QuoteMeta(prefix) + `[A-Za-z0-9]{` + fmt.Sprintf("%d", xCount) + `}`,
	)
	replacement := []byte(prefix + strings.Repeat("X", xCount))
	return func(b []byte) []byte {
		return pattern.ReplaceAll(b, replacement)
	}
}

var dryRunWarningRe = regexp.MustCompile(`(?m)^[^\n]*warning:[^\n]*\n?`)

func normalizeDryRunWarning(b []byte) []byte {
	return dryRunWarningRe.ReplaceAll(b, nil)
}

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?mktemp\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("mktemp"))
}

func verifyCreated(t *testing.T, dir, prefix string, suffixLen int, mode os.FileMode, expectDir bool) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	found := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		found++
		suffix := name[len(prefix):]
		if len(suffix) != suffixLen {
			t.Errorf("entry %q: expected %d random chars, got %d", name, suffixLen, len(suffix))
		}
		for _, c := range suffix {
			if !isAlphanumeric(c) {
				t.Errorf("entry %q: non-alphanumeric char %q in suffix", name, c)
			}
		}
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("stat %q: %v", name, err)
		}
		if expectDir {
			if !info.IsDir() {
				t.Errorf("entry %q: expected directory", name)
			}
		} else {
			if !info.Mode().IsRegular() {
				t.Errorf("entry %q: expected regular file", name)
			}
		}
		if perm := info.Mode().Perm(); perm != mode {
			t.Errorf("entry %q: expected mode %04o, got %04o", name, mode, perm)
		}
	}
	if found == 0 {
		t.Fatal("no matching entries created in temp dir")
	}
}

func verifyCreatedWithSuffix(
	t *testing.T, dir, prefix string, randomLen int, fileSuffix string,
	mode os.FileMode, expectDir bool,
) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	found := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, fileSuffix) {
			continue
		}
		found++
		middle := name[len(prefix) : len(name)-len(fileSuffix)]
		if len(middle) != randomLen {
			t.Errorf("entry %q: expected %d random chars, got %d", name, randomLen, len(middle))
		}
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("stat %q: %v", name, err)
		}
		if expectDir && !info.IsDir() {
			t.Errorf("entry %q: expected directory", name)
		}
		if perm := info.Mode().Perm(); perm != mode {
			t.Errorf("entry %q: expected mode %04o, got %04o", name, mode, perm)
		}
	}
	if found == 0 {
		t.Fatal("no matching entries created in temp dir")
	}
}

func verifySuffix(t *testing.T, dir, expectedSuffix string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), expectedSuffix) {
			return
		}
	}
	t.Fatalf("no entry with suffix %q found in %s", expectedSuffix, dir)
}

func verifyNotCreated(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected no files in %s, found: %v", dir, names)
	}
}

func isAlphanumeric(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
