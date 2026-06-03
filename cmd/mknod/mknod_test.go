// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements srd093-mknod R1.1, R1.2, R1.3, R2.1, R2.2, R2.3.
package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

var binaryNameRe = regexp.MustCompile(`(/\S+/)?g?mknod\b`)

func normalizeBinaryName(b []byte) []byte {
	return binaryNameRe.ReplaceAll(b, []byte("mknod"))
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
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

	t.Run("missing_type", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "name_only",
				Args:      []string{"foo"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("invalid_type", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bad_type",
				Args:      []string{"foo", "x"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("fifo_create", func(t *testing.T) {
		runFifoTest(t, goBin, refBin, []string{"pipe1", "p"})
	})

	t.Run("fifo_with_mode", func(t *testing.T) {
		runFifoTest(t, goBin, refBin, []string{"-m", "0600", "pipe1", "p"})
	})

	t.Run("fifo_mode_long", func(t *testing.T) {
		runFifoTest(t, goBin, refBin, []string{"--mode=0644", "pipe1", "p"})
	})

	t.Run("fifo_mode_symbolic", func(t *testing.T) {
		runFifoTest(t, goBin, refBin, []string{"-m", "u=rw,go=r", "pipe1", "p"})
	})

	t.Run("fifo_extra_operand", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "p_with_major",
				Args:      []string{"foo", "p", "1"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("block_missing_minor", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "b_no_minor",
				Args:      []string{"foo", "b", "1"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("block_missing_both", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "b_no_devnums",
				Args:      []string{"foo", "b"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("char_missing_minor", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "c_no_minor",
				Args:      []string{"foo", "c", "1"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("invalid_major", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "nonnumeric_major",
				Args:      []string{"foo", "b", "abc", "0"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("invalid_minor", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "nonnumeric_minor",
				Args:      []string{"foo", "b", "1", "abc"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("fifo_exists", func(t *testing.T) {
		workDir := t.TempDir()
		os.WriteFile(filepath.Join(workDir, "existing"), nil, 0o644)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "exists",
				Args:      []string{"existing", "p"},
				WorkDir:   workDir,
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("fifo_no_parent", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "missing_dir",
				Args:      []string{"no/parent/pipe", "p"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("invalid_option", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bad_option",
				Args:      []string{"-x", "foo", "p"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("mode_invalid", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bogus_mode",
				Args:      []string{"-m", "bogus", "foo", "p"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("block_extra_operand", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "b_extra",
				Args:      []string{"foo", "b", "1", "2", "extra"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("invalid_type_with_devnums", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bad_type_full_args",
				Args:      []string{"foo", "x", "1", "2"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("permission_denied", func(t *testing.T) {
		workDir := t.TempDir()
		roDir := filepath.Join(workDir, "readonly")
		os.Mkdir(roDir, 0o555)
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "no_write_perm",
				Args:      []string{filepath.Join(roDir, "pipe"), "p"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("mode_missing_arg_short", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "m_no_arg",
				Args:      []string{"-m"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})

	t.Run("unrecognized_long_option", func(t *testing.T) {
		testutils.RunDiffTests(t, goBin, refBin, []testutils.DiffTest{
			{
				Name:      "bad_long_opt",
				Args:      []string{"--foo", "bar", "p"},
				ExitCode:  1,
				Normalize: []testutils.NormalizeFunc{normalizeBinaryName},
			},
		})
	})
}

func TestFifoCreation(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary not found")
	}

	goDir := t.TempDir()
	refDir := t.TempDir()
	args := []string{"testpipe", "p"}
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

func TestFifoModePermissions(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmknod")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []struct {
		name string
		args []string
	}{
		{"octal_0600", []string{"-m", "0600", "p1", "p"}},
		{"octal_0644", []string{"-m", "0644", "p2", "p"}},
		{"octal_0666", []string{"-m", "0666", "p3", "p"}},
		{"symbolic_urw_gor", []string{"-m", "u=rw,go=r", "p4", "p"}},
		{"symbolic_arw", []string{"-m", "a=rw", "p5", "p"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goDir := t.TempDir()
			refDir := t.TempDir()
			runBin(t, goBin, tt.args, goDir)
			runBin(t, refBin, tt.args, refDir)
			name := tt.args[len(tt.args)-2]
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

func runFifoTest(t *testing.T, goBin, refBin string, args []string) {
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

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "--help")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("mknod timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}
