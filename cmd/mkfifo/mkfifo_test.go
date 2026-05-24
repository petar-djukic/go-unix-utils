// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd092-mkfifo R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
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

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?mkfifo\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("mkfifo"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary not found")
	}

	t.Run("no_args", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "missing_operand",
				Args:      []string{},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("create_single", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"pipe1"})
	})

	t.Run("create_multiple", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"p1", "p2", "p3"})
	})

	t.Run("mode_octal", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-m", "0600", "pipe1"})
	})

	t.Run("mode_octal_no_leading_zero", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-m", "600", "pipe1"})
	})

	t.Run("mode_long_flag", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"--mode=0600", "pipe1"})
	})

	t.Run("mode_long_flag_separate", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"--mode", "0600", "pipe1"})
	})

	t.Run("mode_combined_short", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-m0600", "pipe1"})
	})

	t.Run("mode_symbolic", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"-m", "u=rw,go=r", "pipe1"})
	})

	t.Run("error_existing", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "existing"), nil, 0o644)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "exists",
				Args:      []string{"existing"},
				WorkDir:   workDir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("error_missing_parent", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no_parent",
				Args:      []string{"no/parent/pipe"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("partial_failure", func(t *testing.T) {
		goDir := t.TempDir()
		refDir := t.TempDir()
		os.WriteFile(filepath.Join(goDir, "existing"), nil, 0o644)
		os.WriteFile(filepath.Join(refDir, "existing"), nil, 0o644)
		args := []string{"existing", "newpipe"}
		goRes := runBin(t, goBin, args, goDir)
		refRes := runBin(t, refBin, args, refDir)
		compareResults(t, args, goRes, refRes)
	})

	t.Run("mode_invalid", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bogus_mode",
				Args:      []string{"-m", "bogus", "pipe"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("mode_missing_arg", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "m_no_arg",
				Args:      []string{"-m"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("invalid_option", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bad_short_option",
				Args:      []string{"-x"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("double_dash", func(t *testing.T) {
		runCreationTest(t, goBin, refBin, []string{"--", "-pipename"})
	})
}

func TestCreationPermissions(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary not found")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	args := []string{"testpipe"}
	runBin(t, goBin, args, goDir)
	runBin(t, refBin, args, refDir)

	goInfo, err := os.Stat(filepath.Join(goDir, "testpipe"))
	if err != nil {
		t.Fatalf("go binary did not create FIFO: %v", err)
	}
	refInfo, err := os.Stat(filepath.Join(refDir, "testpipe"))
	if err != nil {
		t.Fatalf("ref binary did not create FIFO: %v", err)
	}
	if goInfo.Mode().Type()&os.ModeNamedPipe == 0 {
		t.Fatal("go binary created a non-FIFO file")
	}
	if goInfo.Mode().Perm() != refInfo.Mode().Perm() {
		t.Fatalf("permission mismatch: go=%v ref=%v",
			goInfo.Mode().Perm(), refInfo.Mode().Perm())
	}
}

func TestModePermissions(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmkfifo")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []struct {
		name string
		args []string
	}{
		{"octal_0600", []string{"-m", "0600", "p1"}},
		{"octal_0644", []string{"-m", "0644", "p2"}},
		{"octal_0666", []string{"-m", "0666", "p3"}},
		{"symbolic_urw_gor", []string{"-m", "u=rw,go=r", "p4"}},
		{"symbolic_arw", []string{"-m", "a=rw", "p5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goDir := t.TempDir()
			refDir := t.TempDir()
			runBin(t, goBin, tt.args, goDir)
			runBin(t, refBin, tt.args, refDir)
			name := tt.args[len(tt.args)-1]
			goInfo, err := os.Stat(filepath.Join(goDir, name))
			if err != nil {
				t.Fatalf("go binary did not create %s: %v", name, err)
			}
			refInfo, err := os.Stat(filepath.Join(refDir, name))
			if err != nil {
				t.Fatalf("ref binary did not create %s: %v", name, err)
			}
			if goInfo.Mode().Perm() != refInfo.Mode().Perm() {
				t.Fatalf("permission mismatch on %s: go=%v ref=%v",
					name, goInfo.Mode().Perm(), refInfo.Mode().Perm())
			}
		})
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
