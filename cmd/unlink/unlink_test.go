// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/unlink against the GNU reference binary (gunlink).
//
// Implements prd038-unlink acceptance criteria AC1-AC6 via a custom differential
// test helper. A custom helper is required because unlink removes files, and the
// standard RunDiffTests harness shares a single workdir between both binaries —
// the reference binary would remove the file first, causing the Go binary to
// fail on a missing file.
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gunlink")
	if err != nil {
		t.Skipf("reference binary gunlink not in PATH: %v", err)
	}

	// R1.1: Remove a regular file.
	t.Run("unlink_file", func(t *testing.T) {
		t.Parallel()
		runUnlinkDiff(t, goBin, refBin, []string{"target.txt"}, func(dir string) {
			os.WriteFile(filepath.Join(dir, "target.txt"), []byte("content"), 0644) //nolint:errcheck // test setup
		})
	})

	// R3.2: Remove a symbolic link.
	t.Run("unlink_symlink", func(t *testing.T) {
		t.Parallel()
		runUnlinkDiff(t, goBin, refBin, []string{"link.txt"}, func(dir string) {
			os.WriteFile(filepath.Join(dir, "real.txt"), []byte("content"), 0644) //nolint:errcheck // test setup
			os.Symlink(filepath.Join(dir, "real.txt"), filepath.Join(dir, "link.txt"))  //nolint:errcheck // test setup
		})
	})

	// R2.1: No arguments — error.
	t.Run("unlink_no_args", func(t *testing.T) {
		t.Parallel()
		runUnlinkDiff(t, goBin, refBin, nil, nil)
	})

	// R2.2: Extra operand — error.
	t.Run("unlink_extra_operand", func(t *testing.T) {
		t.Parallel()
		runUnlinkDiff(t, goBin, refBin, []string{"a", "b"}, nil)
	})

	// R2.3: Nonexistent file — error.
	t.Run("unlink_nonexistent", func(t *testing.T) {
		t.Parallel()
		runUnlinkDiff(t, goBin, refBin, []string{"nonexistent.txt"}, nil)
	})

	// R2.4: Directory argument — error.
	t.Run("unlink_directory", func(t *testing.T) {
		t.Parallel()
		runUnlinkDiff(t, goBin, refBin, []string{"somedir"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "somedir"), 0755) //nolint:errcheck // test setup
		})
	})

	// Permission denied scenario.
	t.Run("unlink_permission_denied", func(t *testing.T) {
		t.Parallel()
		runUnlinkDiff(t, goBin, refBin, []string{"protected/target.txt"}, func(dir string) {
			protDir := filepath.Join(dir, "protected")
			os.Mkdir(protDir, 0755)                                              //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(protDir, "target.txt"), []byte("x"), 0644) //nolint:errcheck // test setup
			os.Chmod(protDir, 0555)                                              //nolint:errcheck // test setup
		})
	})

	// R3.3: Verify file no longer exists after successful removal.
	t.Run("unlink_file_removed", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		target := filepath.Join(dir, "removeme.txt")
		os.WriteFile(target, []byte("data"), 0644) //nolint:errcheck // test setup

		env := buildTestEnv()
		_, _, exitCode := runBinary(t, goBin, []string{"removeme.txt"}, env, dir)
		if exitCode != 0 {
			t.Fatalf("expected exit 0, got %d", exitCode)
		}
		if _, err := os.Stat(target); !os.IsNotExist(err) {
			t.Errorf("file %s still exists after unlink", target)
		}
	})
}

// binaryNameNormalizer replaces the binary name (including full path variants)
// in stdout and stderr so that output from gunlink and our unlink compare equal.
var binaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`(?:/\S+/)?(gunlink)\b`)
	b = re.ReplaceAll(b, []byte("unlink"))
	re2 := regexp.MustCompile(`/\S+/unlink\b`)
	b = re2.ReplaceAll(b, []byte("unlink"))
	return b
}

// runUnlinkDiff runs both binaries in separate temp directories and compares
// stdout, stderr, and exit code. setup (if non-nil) is called for each
// directory before the binary runs to pre-create filesystem state.
func runUnlinkDiff(t *testing.T, goBin, refBin string, args []string, setup func(dir string)) {
	t.Helper()

	refDir := t.TempDir()
	goDir := t.TempDir()

	if setup != nil {
		setup(refDir)
		setup(goDir)
	}

	env := buildTestEnv()

	refStdout, refStderr, refExit := runBinary(t, refBin, args, env, refDir)
	goStdout, goStderr, goExit := runBinary(t, goBin, args, env, goDir)

	// Normalize binary names.
	refStdout = binaryNameNormalizer(refStdout)
	refStderr = binaryNameNormalizer(refStderr)
	goStdout = binaryNameNormalizer(goStdout)
	goStderr = binaryNameNormalizer(goStderr)

	if !bytes.Equal(refStdout, goStdout) || !bytes.Equal(refStderr, goStderr) || refExit != goExit {
		t.Errorf("divergence detected\n"+
			"args:       %v\n"+
			"ref stdout: %q\n"+
			"go  stdout: %q\n"+
			"ref stderr: %q\n"+
			"go  stderr: %q\n"+
			"ref exit:   %d\n"+
			"go  exit:   %d",
			args, refStdout, goStdout, refStderr, goStderr, refExit, goExit)
	}

	// Restore permissions for cleanup if needed.
	if setup != nil {
		filepath.Walk(refDir, func(path string, info os.FileInfo, err error) error { //nolint:errcheck // best-effort cleanup
			if err == nil && info.IsDir() {
				os.Chmod(path, 0755) //nolint:errcheck // best-effort cleanup
			}
			return nil
		})
		filepath.Walk(goDir, func(path string, info os.FileInfo, err error) error { //nolint:errcheck // best-effort cleanup
			if err == nil && info.IsDir() {
				os.Chmod(path, 0755) //nolint:errcheck // best-effort cleanup
			}
			return nil
		})
	}
}

// runBinary executes a binary with the given arguments and returns its output.
func runBinary(t *testing.T, binary string, args []string, env []string, workDir string) (stdout, stderr []byte, exitCode int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = env

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Run()
	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()

	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("binary %s timed out", binary)
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}

	return stdout, stderr, exitCode
}

// buildTestEnv constructs the environment with LC_ALL=C set.
func buildTestEnv() []string {
	env := os.Environ()
	prefix := "LC_ALL="
	found := false
	for i, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[i] = "LC_ALL=C"
			found = true
			break
		}
	}
	if !found {
		env = append(env, "LC_ALL=C")
	}
	return env
}
