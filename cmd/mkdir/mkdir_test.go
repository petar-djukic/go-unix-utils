// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mkdir against the GNU reference binary (gmkdir).
//
// Implements prd034-mkdir acceptance criteria AC1-AC6 via a custom differential
// test helper. A custom helper is required because mkdir creates directories,
// and the standard RunDiffTests harness shares a single workdir between both
// binaries — the reference binary would create the directory first, causing
// the Go binary to fail on the already-existing directory.
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
	refBin, err := exec.LookPath("gmkdir")
	if err != nil {
		t.Skipf("reference binary gmkdir not in PATH: %v", err)
	}

	// R1.1: Create a single directory.
	t.Run("mkdir_single_dir", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"newdir"}, nil)
	})

	// R1.2: Create multiple directories.
	t.Run("mkdir_multiple_dirs", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"dir1", "dir2", "dir3"}, nil)
	})

	// R1.3: Error when directory already exists without -p.
	t.Run("mkdir_exists_error", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"existingdir"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "existingdir"), 0755) //nolint:errcheck // test setup
		})
	})

	// R1.4: Error when parent does not exist without -p.
	t.Run("mkdir_missing_parent_error", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"noparent/child"}, nil)
	})

	// R2.1: -p creates intermediate parent directories.
	t.Run("mkdir_parents", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"-p", "a/b/c"}, nil)
	})

	// R2.2: -p does not error when directory already exists.
	t.Run("mkdir_parents_exists_ok", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"-p", "existingdir"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "existingdir"), 0755) //nolint:errcheck // test setup
		})
	})

	// R2.3: -p does not error when intermediate directories already exist.
	t.Run("mkdir_parents_intermediate_exists", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"-p", "a/b/c"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "a"), 0755) //nolint:errcheck // test setup
		})
	})

	// R3.1: -m sets permissions (octal).
	t.Run("mkdir_mode_octal", func(t *testing.T) {
		t.Parallel()
		runMkdirDiffWithPermCheck(t, goBin, refBin, []string{"-m", "0750", "restricted"}, nil, "restricted", 0750)
	})

	// R3.1: -m with octal no leading zero.
	t.Run("mkdir_mode_octal_no_prefix", func(t *testing.T) {
		t.Parallel()
		runMkdirDiffWithPermCheck(t, goBin, refBin, []string{"-m", "755", "modedir"}, nil, "modedir", 0755)
	})

	// R3.3: -p -m applies mode only to final directory.
	t.Run("mkdir_parents_mode", func(t *testing.T) {
		t.Parallel()
		runMkdirDiffWithPermCheck(t, goBin, refBin, []string{"-p", "-m", "0700", "x/y/z"}, nil, "x/y/z", 0700)
	})

	// R3.4: -v verbose output.
	t.Run("mkdir_verbose", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"-v", "verbosedir"}, nil)
	})

	// R3.4: -p -v verbose output for each created directory.
	t.Run("mkdir_verbose_parents", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"-pv", "p/q/r"}, nil)
	})

	// Combined: -m -v.
	t.Run("mkdir_mode_verbose", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"-v", "-m", "0755", "mvdir"}, nil)
	})

	// R1.2: Multiple dirs with one failing (existing) — continues processing.
	t.Run("mkdir_multiple_partial_error", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"existing", "newone"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "existing"), 0755) //nolint:errcheck // test setup
		})
	})

	// --parents long option form.
	t.Run("mkdir_long_parents", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, []string{"--parents", "long/path/dir"}, nil)
	})

	// --mode=OCTAL long option form.
	t.Run("mkdir_long_mode", func(t *testing.T) {
		t.Parallel()
		runMkdirDiffWithPermCheck(t, goBin, refBin, []string{"--mode=0700", "longmodedir"}, nil, "longmodedir", 0700)
	})

	// No arguments — error.
	t.Run("mkdir_no_args", func(t *testing.T) {
		t.Parallel()
		runMkdirDiff(t, goBin, refBin, nil, nil)
	})
}

// binaryNameNormalizer replaces the binary name (including full path variants)
// in stdout and stderr so that output from gmkdir and our mkdir compare equal.
// GNU coreutils may use the full argv[0] path in verbose and help output.
var binaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	// Replace /path/to/gmkdir or /path/to/mkdir with just "mkdir".
	re := regexp.MustCompile(`(?:/\S+/)?(gmkdir)\b`)
	b = re.ReplaceAll(b, []byte("mkdir"))
	// Also handle /full/path/mkdir (resolved symlink).
	re2 := regexp.MustCompile(`/\S+/mkdir\b`)
	b = re2.ReplaceAll(b, []byte("mkdir"))
	return b
}

// runMkdirDiff runs both binaries in separate temp directories and compares
// stdout, stderr, and exit code. setup (if non-nil) is called for each
// directory before the binary runs to pre-create filesystem state.
func runMkdirDiff(t *testing.T, goBin, refBin string, args []string, setup func(dir string)) {
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

// runMkdirDiffWithPermCheck is like runMkdirDiff but additionally verifies
// that the created directory has the expected permission bits. R4.3.
func runMkdirDiffWithPermCheck(t *testing.T, goBin, refBin string, args []string, setup func(dir string), checkPath string, expectedPerm os.FileMode) {
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

	// Verify permission bits match between ref and Go outputs.
	refInfo, refErr := os.Stat(filepath.Join(refDir, checkPath))
	goInfo, goErr := os.Stat(filepath.Join(goDir, checkPath))

	if refErr != nil || goErr != nil {
		if refErr != nil {
			t.Errorf("ref directory %s not found: %v", checkPath, refErr)
		}
		if goErr != nil {
			t.Errorf("go directory %s not found: %v", checkPath, goErr)
		}
		return
	}

	refMode := refInfo.Mode().Perm()
	goMode := goInfo.Mode().Perm()
	if refMode != goMode {
		t.Errorf("permission mismatch for %s: ref=%04o go=%04o", checkPath, refMode, goMode)
	}
	if goMode != expectedPerm {
		t.Errorf("permission mismatch for %s: expected=%04o got=%04o", checkPath, expectedPerm, goMode)
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
