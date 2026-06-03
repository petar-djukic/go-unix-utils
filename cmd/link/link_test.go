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
	"syscall"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var binaryNameRe = regexp.MustCompile(`(?:/\S+/)?g?link`)

func normBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("link"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("glink")
	if err != nil {
		t.Skip("reference binary not found")
	}

	discardStdout := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	t.Run("create-hard-link", func(t *testing.T) {
		runLinkTest(t, goBin, refBin, []string{"source", "target"},
			func(dir string) {
				os.WriteFile(filepath.Join(dir, "source"), []byte("hello"), 0o644)
			})
	})

	t.Run("zero-args", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "zero-args",
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("one-arg", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "one-arg",
				Args:      []string{"file1"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("extra-operand", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "extra-operand",
				Args:      []string{"a", "b", "c"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("nonexistent-source", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "nonexistent-source",
				Args:      []string{"nosuchfile", "newlink"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("target-exists", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "source"), []byte("x"), 0o644)
		os.WriteFile(filepath.Join(workDir, "target"), []byte("y"), 0o644)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "target-exists",
				Args:      []string{"source", "target"},
				WorkDir:   workDir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normBinaryName},
			},
		})
	})

	t.Run("help", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "help",
				Args:      []string{"--help"},
				Normalize: []testutils.NormalizeFunc{discardStdout},
			},
		})
	})

	t.Run("version", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "version",
				Args:      []string{"--version"},
				Normalize: []testutils.NormalizeFunc{discardStdout},
			},
		})
	})
}

type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

func runLinkTest(
	t *testing.T, goBin, refBin string,
	args []string, setup func(string),
) {
	t.Helper()
	goDir := t.TempDir()
	refDir := t.TempDir()
	setup(goDir)
	setup(refDir)
	goRes := runBin(t, goBin, args, goDir)
	refRes := runBin(t, refBin, args, refDir)
	compareResults(t, args, goRes, refRes)

	source := filepath.Join(goDir, args[0])
	target := filepath.Join(goDir, args[1])
	var srcStat, tgtStat syscall.Stat_t
	if err := syscall.Stat(source, &srcStat); err != nil {
		t.Fatalf("source file missing after link: %v", err)
	}
	if err := syscall.Stat(target, &tgtStat); err != nil {
		t.Fatalf("target file missing after link: %v", err)
	}
	if srcStat.Ino != tgtStat.Ino {
		t.Fatalf("expected same inode, got source=%d target=%d", srcStat.Ino, tgtStat.Ino)
	}
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
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to start binary %s: %v", binary, err)
		}
	}
	return binResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitCode}
}

func compareResults(t *testing.T, args []string, goRes, refRes binResult) {
	t.Helper()
	goStdout := normBinaryName(goRes.stdout)
	refStdout := normBinaryName(refRes.stdout)
	goStderr := normBinaryName(goRes.stderr)
	refStderr := normBinaryName(refRes.stderr)

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
