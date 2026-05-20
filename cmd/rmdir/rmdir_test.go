// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd035-rmdir R4.1, R4.2, R4.3.
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skip("reference binary not found")
	}

	t.Run("no_args", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "missing_operand",
				Args:     []string{},
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("nonexistent", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "no_such_dir",
				Args:     []string{"no_such_dir"},
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("not_a_directory", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "afile"), []byte("content"), 0o644)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "file_not_dir",
				Args:     []string{"afile"},
				WorkDir:  workDir,
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("nonempty", func(t *testing.T) {
		workDir := t.TempDir()
		os.Mkdir(filepath.Join(workDir, "nonempty"), 0o755)
		os.WriteFile(filepath.Join(workDir, "nonempty", "file"), []byte("x"), 0o644)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "not_empty",
				Args:     []string{"nonempty"},
				WorkDir:  workDir,
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("single_empty", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"emptydir"},
			func(dir string) {
				os.Mkdir(filepath.Join(dir, "emptydir"), 0o755)
			})
	})

	t.Run("multiple_empty", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"aaa", "bbb", "ccc"},
			func(dir string) {
				os.Mkdir(filepath.Join(dir, "aaa"), 0o755)
				os.Mkdir(filepath.Join(dir, "bbb"), 0o755)
				os.Mkdir(filepath.Join(dir, "ccc"), 0o755)
			})
	})

	t.Run("partial_failure", func(t *testing.T) {
		setup := func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0o755)
			os.Mkdir(filepath.Join(dir, "nonempty"), 0o755)
			os.WriteFile(filepath.Join(dir, "nonempty", "file"), []byte("x"), 0o644)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"emptydir", "nonempty"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("multiple_nonexistent", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "two_missing",
				Args:     []string{"nope1", "nope2"},
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("parents_nested_empty", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-p", "a/b/c"},
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)
			})
	})

	t.Run("parents_stop_at_nonempty", func(t *testing.T) {
		setup := func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)
			os.WriteFile(filepath.Join(dir, "a", "other"), []byte("x"), 0o644)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"-p", "a/b/c"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("parents_multiple_args", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-p", "x/y", "p/q"},
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "x", "y"), 0o755)
				os.MkdirAll(filepath.Join(dir, "p", "q"), 0o755)
			})
	})

	t.Run("parents_single_dir", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-p", "lonely"},
			func(dir string) {
				os.Mkdir(filepath.Join(dir, "lonely"), 0o755)
			})
	})

	t.Run("parents_nonexistent", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "parents_no_such_dir",
				Args:     []string{"-p", "no_such_dir"},
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("ignore_nonempty", func(t *testing.T) {
		setup := func(dir string) {
			os.Mkdir(filepath.Join(dir, "nonempty"), 0o755)
			os.WriteFile(filepath.Join(dir, "nonempty", "file"), []byte("x"), 0o644)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"--ignore-fail-on-non-empty", "nonempty"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("ignore_nonexistent", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "ignore_missing",
				Args:     []string{"--ignore-fail-on-non-empty", "no_such_dir"},
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("ignore_with_parents_nonempty_ancestor", func(t *testing.T) {
		setup := func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)
			os.WriteFile(filepath.Join(dir, "a", "other"), []byte("x"), 0o644)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"--ignore-fail-on-non-empty", "-p", "a/b/c"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("verbose_single", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-v", "emptydir"},
			func(dir string) {
				os.Mkdir(filepath.Join(dir, "emptydir"), 0o755)
			})
	})

	t.Run("verbose_multiple", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-v", "aaa", "bbb"},
			func(dir string) {
				os.Mkdir(filepath.Join(dir, "aaa"), 0o755)
				os.Mkdir(filepath.Join(dir, "bbb"), 0o755)
			})
	})

	t.Run("verbose_parents", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"-v", "-p", "a/b/c"},
			func(dir string) {
				os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)
			})
	})

	t.Run("verbose_with_ignore", func(t *testing.T) {
		setup := func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0o755)
			os.Mkdir(filepath.Join(dir, "nonempty"), 0o755)
			os.WriteFile(filepath.Join(dir, "nonempty", "file"), []byte("x"), 0o644)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"-v", "--ignore-fail-on-non-empty", "emptydir", "nonempty"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})
}

func runRemovalTest(t *testing.T, goBin, refBin string, args []string, setup func(string)) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
	setup(goDir)
	setup(refDir)
	goRes := runBin(t, goBin, args, goDir)
	refRes := runBin(t, refBin, args, refDir)
	compareResults(t, args, goRes, refRes)
}

type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

func runBin(t *testing.T, binary string, args []string, workDir string) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", binary)
	}
	exitCode := 0
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to start binary %s: %v", binary, err)
		}
	}
	return binResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitCode}
}

func compareResults(t *testing.T, args []string, goRes, refRes binResult) {
	t.Helper()
	goStdout := normalizeBinaryName(goRes.stdout)
	refStdout := normalizeBinaryName(refRes.stdout)
	goStderr := normalizeBinaryName(goRes.stderr)
	refStderr := normalizeBinaryName(refRes.stderr)

	if !bytes.Equal(goStdout, refStdout) ||
		!bytes.Equal(goStderr, refStderr) ||
		goRes.exitCode != refRes.exitCode {
		t.Fatalf("divergence detected\n"+
			"  args:       %v\n"+
			"  ref stdout: %q\n"+
			"  go  stdout: %q\n"+
			"  ref stderr: %q\n"+
			"  go  stderr: %q\n"+
			"  ref exit:   %d\n"+
			"  go  exit:   %d\n",
			args, refStdout, goStdout, refStderr, goStderr, refRes.exitCode, goRes.exitCode)
	}
}

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?rmdir\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("rmdir"))
}
