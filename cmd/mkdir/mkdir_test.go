// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd034-mkdir R2.1, R2.2, R2.3, R4.1, R4.2, R4.3.
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
	refBin, err := exec.LookPath("gmkdir")
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

	t.Run("error_existing_dir", func(t *testing.T) {
		workDir := t.TempDir()
		os.Mkdir(filepath.Join(workDir, "existdir"), 0o755)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "exists",
				Args:     []string{"existdir"},
				WorkDir:  workDir,
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("error_missing_parent", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "no_parent",
				Args:     []string{"no/parent/here"},
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("error_partial_failure", func(t *testing.T) {
		workDir := t.TempDir()
		os.Mkdir(filepath.Join(workDir, "existdir"), 0o755)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:     "one_exists_one_missing_parent",
				Args:     []string{"existdir", "no/parent"},
				WorkDir:  workDir,
				ExitCode: 1,
				Normalize: []testutils.NormalizeFunc{
					normalizeBinaryName,
				},
			},
		})
	})

	t.Run("create_single", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"newdir"})
	})

	t.Run("create_multiple", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"aaa", "bbb", "ccc"})
	})

	t.Run("verbose_single", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-v", "vdir"})
	})

	t.Run("verbose_multiple", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-v", "x", "y", "z"})
	})

	t.Run("verbose_long", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"--verbose", "longvdir"})
	})

	t.Run("parents_nested", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-p", "a/b/c"})
	})

	t.Run("parents_existing_target", func(t *testing.T) {
		goDir := t.TempDir()
		refDir := t.TempDir()
		os.Mkdir(filepath.Join(goDir, "existdir"), 0o755)
		os.Mkdir(filepath.Join(refDir, "existdir"), 0o755)
		args := []string{"-p", "existdir"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("parents_partial_existing", func(t *testing.T) {
		goDir := t.TempDir()
		refDir := t.TempDir()
		os.Mkdir(filepath.Join(goDir, "a"), 0o755)
		os.Mkdir(filepath.Join(refDir, "a"), 0o755)
		args := []string{"-p", "a/b/c"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("parents_long_flag", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"--parents", "x/y/z"})
	})

	t.Run("parents_verbose", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-p", "-v", "d/e/f"})
	})

	t.Run("parents_verbose_combined", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-pv", "g/h/i"})
	})

	t.Run("parents_multiple", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-p", "m1/m2", "n1/n2"})
	})

	t.Run("verbose_partial_failure", func(t *testing.T) {
		goDir := t.TempDir()
		refDir := t.TempDir()
		os.Mkdir(filepath.Join(goDir, "existdir"), 0o755)
		os.Mkdir(filepath.Join(refDir, "existdir"), 0o755)
		args := []string{"-v", "existdir", "newdir"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})
}

func TestCreationPermissions(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary not found")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	args := []string{"permdir"}
	runBin(t, goBin, args, goDir)
	runBin(t, refBin, args, refDir)

	goInfo, err := os.Stat(filepath.Join(goDir, "permdir"))
	if err != nil {
		t.Fatalf("go binary did not create directory: %v", err)
	}
	refInfo, err := os.Stat(filepath.Join(refDir, "permdir"))
	if err != nil {
		t.Fatalf("ref binary did not create directory: %v", err)
	}
	if goInfo.Mode().Perm() != refInfo.Mode().Perm() {
		t.Fatalf("permission mismatch: go=%v ref=%v",
			goInfo.Mode().Perm(), refInfo.Mode().Perm())
	}
}

func runCreationTest(t *testing.T, goBin, refBin string, args []string) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
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

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?mkdir\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("mkdir"))
}

func TestParentsPermissions(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary not found")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	args := []string{"-p", "a/b/c"}
	runBin(t, goBin, args, goDir)
	runBin(t, refBin, args, refDir)

	for _, sub := range []string{"a", "a/b", "a/b/c"} {
		goInfo, err := os.Stat(filepath.Join(goDir, sub))
		if err != nil {
			t.Fatalf("go binary did not create %s: %v", sub, err)
		}
		refInfo, err := os.Stat(filepath.Join(refDir, sub))
		if err != nil {
			t.Fatalf("ref binary did not create %s: %v", sub, err)
		}
		if goInfo.Mode().Perm() != refInfo.Mode().Perm() {
			t.Fatalf("permission mismatch on %s: go=%v ref=%v",
				sub, goInfo.Mode().Perm(), refInfo.Mode().Perm())
		}
	}
}

func TestUnrecognizedOption(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skip("reference binary not found")
	}

	testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
		{
			Name:     "bad_long_option",
			Args:     []string{"--badopt"},
			ExitCode: 1,
			Normalize: []testutils.NormalizeFunc{
				normalizeBinaryName,
			},
		},
	})
}
