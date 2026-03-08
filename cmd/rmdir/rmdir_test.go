// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rmdir against the GNU reference binary (grmdir).
//
// Implements prd035-rmdir acceptance criteria AC1-AC5 via a custom differential
// test helper. A custom helper is required because rmdir removes directories,
// and the standard RunDiffTests harness shares a single workdir between both
// binaries — the reference binary would remove the directory first, causing
// the Go binary to fail on a missing directory.
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
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	// R1.1: Remove a single empty directory.
	t.Run("rmdir_single_empty", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"emptydir"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0755) //nolint:errcheck // test setup
		})
	})

	// R1.2: Remove multiple empty directories.
	t.Run("rmdir_multiple_dirs", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"dir1", "dir2", "dir3"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "dir1"), 0755) //nolint:errcheck // test setup
			os.Mkdir(filepath.Join(dir, "dir2"), 0755) //nolint:errcheck // test setup
			os.Mkdir(filepath.Join(dir, "dir3"), 0755) //nolint:errcheck // test setup
		})
	})

	// R1.3: Error when directory is not empty.
	t.Run("rmdir_non_empty_error", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"nonempty"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "nonempty"), 0755)          //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "nonempty/file"), nil, 0644) //nolint:errcheck // test setup
		})
	})

	// R1.4: Error when directory does not exist.
	t.Run("rmdir_nonexistent_error", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"nonexistent"}, nil)
	})

	// R2.1: -p removes target and empty ancestors.
	t.Run("rmdir_parents", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"-p", "a/b/c"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a/b/c"), 0755) //nolint:errcheck // test setup
		})
	})

	// R2.2: -p stops ascending when parent is not empty.
	t.Run("rmdir_parents_stop_nonempty", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"-p", "a/b/c"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a/b/c"), 0755)          //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a/otherfile"), nil, 0644) //nolint:errcheck // test setup
		})
	})

	// R3.1: --ignore-fail-on-non-empty suppresses non-empty errors.
	t.Run("rmdir_ignore_non_empty", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"--ignore-fail-on-non-empty", "nonempty"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "nonempty"), 0755)          //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "nonempty/file"), nil, 0644) //nolint:errcheck // test setup
		})
	})

	// R3.2: --ignore-fail-on-non-empty does not suppress non-existent errors.
	t.Run("rmdir_ignore_nonexistent_still_errors", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"--ignore-fail-on-non-empty", "nonexistent"}, nil)
	})

	// R3.3: -v verbose output.
	t.Run("rmdir_verbose", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"-v", "emptydir"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0755) //nolint:errcheck // test setup
		})
	})

	// R3.3: -p -v verbose output for each removed directory.
	t.Run("rmdir_verbose_parents", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"-pv", "a/b/c"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a/b/c"), 0755) //nolint:errcheck // test setup
		})
	})

	// R1.2: Multiple dirs with one failing — continues processing.
	t.Run("rmdir_multiple_partial_error", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"nonempty", "emptydir"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "nonempty"), 0755)          //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "nonempty/file"), nil, 0644) //nolint:errcheck // test setup
			os.Mkdir(filepath.Join(dir, "emptydir"), 0755)          //nolint:errcheck // test setup
		})
	})

	// --parents long option form.
	t.Run("rmdir_long_parents", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"--parents", "x/y/z"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "x/y/z"), 0755) //nolint:errcheck // test setup
		})
	})

	// No arguments — error.
	t.Run("rmdir_no_args", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, nil, nil)
	})

	// R1.4: Attempt to remove a regular file.
	t.Run("rmdir_file_not_dir", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"afile"}, func(dir string) {
			os.WriteFile(filepath.Join(dir, "afile"), []byte("data"), 0644) //nolint:errcheck // test setup
		})
	})

	// Combined: --ignore-fail-on-non-empty with -p where an ancestor is non-empty.
	t.Run("rmdir_parents_ignore_non_empty", func(t *testing.T) {
		t.Parallel()
		runRmdirDiff(t, goBin, refBin, []string{"-p", "--ignore-fail-on-non-empty", "a/b/c"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a/b/c"), 0755)          //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a/otherfile"), nil, 0644) //nolint:errcheck // test setup
		})
	})
}

// binaryNameNormalizer replaces the binary name (including full path variants)
// in stdout and stderr so that output from grmdir and our rmdir compare equal.
var binaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`(?:/\S+/)?(grmdir)\b`)
	b = re.ReplaceAll(b, []byte("rmdir"))
	re2 := regexp.MustCompile(`/\S+/rmdir\b`)
	b = re2.ReplaceAll(b, []byte("rmdir"))
	return b
}

// runRmdirDiff runs both binaries in separate temp directories and compares
// stdout, stderr, and exit code. setup (if non-nil) is called for each
// directory before the binary runs to pre-create filesystem state.
func runRmdirDiff(t *testing.T, goBin, refBin string, args []string, setup func(dir string)) {
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
