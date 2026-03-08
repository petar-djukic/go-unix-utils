// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ln against the GNU reference binary (gln).
//
// Implements prd037-ln acceptance criteria AC1-AC5 via a custom differential
// test helper. A custom helper is required because ln creates links, and the
// standard RunDiffTests harness shares a single workdir between both binaries —
// the reference binary would create the link first, causing the Go binary to
// fail on the already-existing link.
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
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// R1.1: Create a hard link to an existing file.
	t.Run("ln_hard_link", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"target.txt", "hardlink.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
		})
	})

	// R1.2: Create hard links for multiple targets into a directory.
	t.Run("ln_multiple_into_dir", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"a.txt", "b.txt", "dest"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "a.txt"), "aaa\n")
			writeTestFile(t, filepath.Join(dir, "b.txt"), "bbb\n")
			os.Mkdir(filepath.Join(dir, "dest"), 0755) //nolint:errcheck // test setup
		})
	})

	// R1.3: Error when hard linking a directory.
	t.Run("ln_hard_link_dir_error", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"somedir", "link"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "somedir"), 0755) //nolint:errcheck // test setup
		})
	})

	// R1.4: Error when link name already exists without -f.
	t.Run("ln_exists_error", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"target.txt", "existing.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
			writeTestFile(t, filepath.Join(dir, "existing.txt"), "old\n")
		})
	})

	// R2.1: -s creates a symbolic link.
	t.Run("ln_symbolic_link", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-s", "target.txt", "symlink.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
		})
	})

	// R2.2: Symbolic links to directories.
	t.Run("ln_symbolic_dir", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-s", "somedir", "dirlink"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "somedir"), 0755) //nolint:errcheck // test setup
		})
	})

	// R3.1: -f removes existing destination before creating link.
	t.Run("ln_force_overwrite", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-sf", "target.txt", "existing.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
			writeTestFile(t, filepath.Join(dir, "existing.txt"), "old\n")
		})
	})

	// R3.1: -f with hard links.
	t.Run("ln_force_hard", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-f", "target.txt", "existing.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
			writeTestFile(t, filepath.Join(dir, "existing.txt"), "old\n")
		})
	})

	// R3.2: -n treats symlink-to-dir as regular file.
	t.Run("ln_no_dereference", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-sfn", "target.txt", "dirlink"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
			os.Mkdir(filepath.Join(dir, "somedir"), 0755) //nolint:errcheck // test setup
			os.Symlink("somedir", filepath.Join(dir, "dirlink")) //nolint:errcheck // test setup
		})
	})

	// R3.4: -v prints verbose output.
	t.Run("ln_verbose_symbolic", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-sv", "target.txt", "link.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
		})
	})

	// R3.4: -v with hard links.
	t.Run("ln_verbose_hard", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-v", "target.txt", "link.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
		})
	})

	// R3.5: -b creates backup before overwrite.
	t.Run("ln_backup", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-sb", "target.txt", "existing.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
			writeTestFile(t, filepath.Join(dir, "existing.txt"), "old\n")
		})
	})

	// R3.6: -S changes backup suffix.
	t.Run("ln_backup_suffix", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-sb", "-S", ".bak", "target.txt", "existing.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
			writeTestFile(t, filepath.Join(dir, "existing.txt"), "old\n")
		})
	})

	// R2.4: -r creates a relative symbolic link.
	t.Run("ln_relative_symlink", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-sr", "subdir/target.txt", "link.txt"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "subdir"), 0755) //nolint:errcheck // test setup
			writeTestFile(t, filepath.Join(dir, "subdir", "target.txt"), "content\n")
		})
	})

	// Error: missing file operand.
	t.Run("ln_no_args", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, nil, nil)
	})

	// Error: missing target for hard link.
	t.Run("ln_missing_target", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"nonexistent.txt", "link.txt"}, nil)
	})

	// Combined: multiple flags -sfv.
	t.Run("ln_force_verbose_symbolic", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"-sfv", "target.txt", "existing.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
			writeTestFile(t, filepath.Join(dir, "existing.txt"), "old\n")
		})
	})

	// Long option forms.
	t.Run("ln_long_symbolic", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"--symbolic", "target.txt", "link.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
		})
	})

	t.Run("ln_long_force", func(t *testing.T) {
		t.Parallel()
		runLnDiff(t, goBin, refBin, []string{"--symbolic", "--force", "target.txt", "existing.txt"}, func(dir string) {
			writeTestFile(t, filepath.Join(dir, "target.txt"), "content\n")
			writeTestFile(t, filepath.Join(dir, "existing.txt"), "old\n")
		})
	})
}

// binaryNameNormalizer replaces the binary name in stderr/stdout so that output
// from gln and our ln compare equal.
var binaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	// Replace /path/to/gln with just "ln".
	re := regexp.MustCompile(`(?:/\S+/)?(gln)\b`)
	b = re.ReplaceAll(b, []byte("ln"))
	// Also handle /full/path/ln (our binary).
	re2 := regexp.MustCompile(`/\S+/ln\b`)
	b = re2.ReplaceAll(b, []byte("ln"))
	return b
}

// runLnDiff runs both binaries in separate temp directories and compares
// stdout, stderr, and exit code. setup (if non-nil) is called for each
// directory before the binary runs to pre-create filesystem state.
func runLnDiff(t *testing.T, goBin, refBin string, args []string, setup func(dir string)) {
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

// writeTestFile creates a file with the given content.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}
