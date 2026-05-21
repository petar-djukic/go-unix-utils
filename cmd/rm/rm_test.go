// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

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
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skip("reference binary grm not found")
	}

	t.Run("dot_rejection", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "refuse_dot",
				Args:      []string{"."},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("dotdot_rejection", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "refuse_dotdot",
				Args:      []string{".."},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("nonexistent_file", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no_such_file",
				Args:      []string{"nonexistent_file"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("directory_without_r", func(t *testing.T) {
		workDir := t.TempDir()
		os.Mkdir(filepath.Join(workDir, "mydir"), 0o755)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "is_a_directory",
				Args:      []string{"mydir"},
				WorkDir:   workDir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("single_file", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"file.txt"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0o644)
			})
	})

	t.Run("multiple_files", func(t *testing.T) {
		runRemovalTest(t, goBin, refBin, []string{"a.txt", "b.txt", "c.txt"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644)
				os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644)
				os.WriteFile(filepath.Join(dir, "c.txt"), []byte("c"), 0o644)
			})
	})

	t.Run("partial_failure_with_nonexistent", func(t *testing.T) {
		setup := func(dir string) {
			os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("data"), 0o644)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"exists.txt", "nonexistent"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("file_and_directory_without_r", func(t *testing.T) {
		setup := func(dir string) {
			os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644)
			os.Mkdir(filepath.Join(dir, "mydir"), 0o755)
		}
		goDir := t.TempDir()
		refDir := t.TempDir()
		setup(goDir)
		setup(refDir)
		args := []string{"file.txt", "mydir"}
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
	goStderr := normalizeBinaryName(goRes.stderr)
	refStderr := normalizeBinaryName(refRes.stderr)
	if !bytes.Equal(goRes.stdout, refRes.stdout) ||
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
			args, refRes.stdout, goRes.stdout, refStderr, goStderr,
			refRes.exitCode, goRes.exitCode)
	}
}

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?rm\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("rm"))
}
