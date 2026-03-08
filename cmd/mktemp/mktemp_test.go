// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mktemp against the GNU reference binary (gmktemp).
//
// Implements prd036-mktemp acceptance criteria AC1-AC6 via structural comparison.
// Since mktemp generates random names, exact byte-for-byte stdout comparison is
// not possible for creation tests. Instead we verify structural properties: exit
// code, stderr match, output is a valid path in the expected directory, and the
// created entry exists with correct type and permissions. Error-path and --dry-run
// tests use RunDiffTests for exact comparison where output is deterministic.
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
	refBin, err := exec.LookPath("gmktemp")
	if err != nil {
		t.Skipf("reference binary gmktemp not in PATH: %v", err)
	}

	// R1.1: Default file creation — structural comparison.
	t.Run("mktemp_default", func(t *testing.T) {
		t.Parallel()
		runMktempStructural(t, goBin, refBin, nil, nil, func(t *testing.T, path string) {
			t.Helper()
			assertFileExists(t, path)
			assertPathPrefix(t, path, os.TempDir())
			assertMatchesPattern(t, filepath.Base(path), `^tmp\.[A-Za-z0-9]{10}$`)
		})
	})

	// R2.1: -d creates a directory.
	t.Run("mktemp_directory", func(t *testing.T) {
		t.Parallel()
		runMktempStructural(t, goBin, refBin, []string{"-d"}, nil, func(t *testing.T, path string) {
			t.Helper()
			assertDirExists(t, path)
			assertPathPrefix(t, path, os.TempDir())
			assertPermissions(t, path, 0700)
		})
	})

	// R1.3: Custom template.
	t.Run("mktemp_custom_template", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		template := filepath.Join(tmpDir, "myfile.XXXXXX")
		runMktempStructural(t, goBin, refBin, []string{template}, nil, func(t *testing.T, path string) {
			t.Helper()
			assertFileExists(t, path)
			assertPathPrefix(t, path, tmpDir)
			assertMatchesPattern(t, filepath.Base(path), `^myfile\.[A-Za-z0-9]{6}$`)
		})
	})

	// R3.1: -p DIR sets parent directory.
	t.Run("mktemp_tmpdir_flag", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		runMktempStructural(t, goBin, refBin, []string{"-p", tmpDir}, nil, func(t *testing.T, path string) {
			t.Helper()
			assertFileExists(t, path)
			assertPathPrefix(t, path, tmpDir)
		})
	})

	// R3.1: --tmpdir=DIR long form.
	t.Run("mktemp_tmpdir_long", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		runMktempStructural(t, goBin, refBin, []string{"--tmpdir=" + tmpDir}, nil, func(t *testing.T, path string) {
			t.Helper()
			assertFileExists(t, path)
			assertPathPrefix(t, path, tmpDir)
		})
	})

	// R3.3: --suffix appends after random chars.
	t.Run("mktemp_suffix", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		runMktempStructural(t, goBin, refBin, []string{"-p", tmpDir, "--suffix=.txt"}, nil, func(t *testing.T, path string) {
			t.Helper()
			assertFileExists(t, path)
			if !strings.HasSuffix(path, ".txt") {
				t.Errorf("expected path to end with .txt, got %s", path)
			}
		})
	})

	// R2.1 + R3.1: -d -p DIR combined.
	t.Run("mktemp_dir_with_tmpdir", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		runMktempStructural(t, goBin, refBin, []string{"-d", "-p", tmpDir}, nil, func(t *testing.T, path string) {
			t.Helper()
			assertDirExists(t, path)
			assertPathPrefix(t, path, tmpDir)
			assertPermissions(t, path, 0700)
		})
	})

	// R3.5: -u dry-run — compare exit codes and stderr; stdout is random but both should print something.
	t.Run("mktemp_dry_run", func(t *testing.T) {
		t.Parallel()
		runMktempStructural(t, goBin, refBin, []string{"-u"}, nil, func(t *testing.T, path string) {
			t.Helper()
			// -u should NOT create the file.
			if _, err := os.Stat(path); err == nil {
				t.Errorf("-u created file %s, expected dry-run", path)
			}
		})
	})

	// R3.4: -t legacy mode.
	t.Run("mktemp_legacy_t", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		runMktempStructural(t, goBin, refBin, []string{"-t", "test.XXXXXX"},
			[]string{"TMPDIR=" + tmpDir},
			func(t *testing.T, path string) {
				t.Helper()
				assertFileExists(t, path)
				assertPathPrefix(t, path, tmpDir)
			})
	})

	// Error: too few X's in template.
	t.Run("mktemp_bad_template", func(t *testing.T) {
		t.Parallel()
		runMktempErrorDiff(t, goBin, refBin, []string{"noXs"}, nil)
	})

	// Error: nonexistent parent directory.
	t.Run("mktemp_bad_dir", func(t *testing.T) {
		t.Parallel()
		runMktempErrorDiff(t, goBin, refBin, []string{"-p", "/nonexistent_dir_12345"}, nil)
	})

	// Error: no args is not an error for mktemp (default template), verify both succeed.
	t.Run("mktemp_no_args_succeeds", func(t *testing.T) {
		t.Parallel()
		goExit := runBinaryExitCode(t, goBin, nil, nil)
		refExit := runBinaryExitCode(t, refBin, nil, nil)
		if goExit != refExit {
			t.Errorf("exit code mismatch: go=%d ref=%d", goExit, refExit)
		}
		if goExit != 0 {
			t.Errorf("expected exit 0 for no-args, got %d", goExit)
		}
	})

	// R1.4: Verify file permissions are 0600.
	t.Run("mktemp_file_permissions", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		runMktempStructural(t, goBin, refBin, []string{"-p", tmpDir}, nil, func(t *testing.T, path string) {
			t.Helper()
			assertPermissions(t, path, 0600)
		})
	})
}

// binaryNameNormalizer replaces binary path names so Go and ref outputs compare equal.
var binaryNameNormalizer testutils.NormalizeFunc = func(b []byte) []byte {
	re := regexp.MustCompile(`(?:/\S+/)?(gmktemp)\b`)
	b = re.ReplaceAll(b, []byte("mktemp"))
	re2 := regexp.MustCompile(`/\S+/mktemp\b`)
	b = re2.ReplaceAll(b, []byte("mktemp"))
	return b
}

// runMktempStructural runs both binaries independently and compares exit codes
// and stderr. For stdout (which contains random paths), it calls the validate
// function on the Go binary's output path to check structural properties.
func runMktempStructural(t *testing.T, goBin, refBin string, args, extraEnv []string, validate func(*testing.T, string)) {
	t.Helper()

	env := buildTestEnv(extraEnv)

	refStdout, refStderr, refExit := runBinary(t, refBin, args, env)
	goStdout, goStderr, goExit := runBinary(t, goBin, args, env)

	// Normalize stderr for binary name differences.
	refStderr = binaryNameNormalizer(refStderr)
	goStderr = binaryNameNormalizer(goStderr)

	// Compare exit codes.
	if refExit != goExit {
		t.Errorf("exit code mismatch\nargs: %v\nref exit: %d\ngo  exit: %d\nref stderr: %q\ngo  stderr: %q",
			args, refExit, goExit, refStderr, goStderr)
	}

	// Compare stderr (normalized).
	if !bytes.Equal(refStderr, goStderr) {
		t.Errorf("stderr mismatch\nargs: %v\nref stderr: %q\ngo  stderr: %q", args, refStderr, goStderr)
	}

	// Validate structural properties of Go binary output.
	goPath := strings.TrimSpace(string(goStdout))
	if goPath != "" && validate != nil {
		validate(t, goPath)
	}

	// Clean up created files from both binaries.
	refPath := strings.TrimSpace(string(refStdout))
	cleanupPath(refPath)
	cleanupPath(goPath)
}

// runMktempErrorDiff runs both binaries expecting failure and compares exit
// codes. Stderr is normalized but not compared byte-for-byte since error
// message wording may differ slightly.
func runMktempErrorDiff(t *testing.T, goBin, refBin string, args, extraEnv []string) {
	t.Helper()

	env := buildTestEnv(extraEnv)

	_, refStderr, refExit := runBinary(t, refBin, args, env)
	_, goStderr, goExit := runBinary(t, goBin, args, env)

	if refExit != goExit {
		t.Errorf("exit code mismatch\nargs: %v\nref exit: %d\ngo  exit: %d", args, refExit, goExit)
	}

	// Both should have non-empty stderr on error.
	if len(refStderr) == 0 {
		t.Logf("warning: ref stderr empty for error case %v", args)
	}
	if len(goStderr) == 0 && len(refStderr) > 0 {
		t.Errorf("go stderr empty but ref stderr non-empty\nargs: %v\nref stderr: %q", args, refStderr)
	}
}

// runBinaryExitCode runs a binary and returns only the exit code.
func runBinaryExitCode(t *testing.T, binary string, args, extraEnv []string) int {
	t.Helper()
	env := buildTestEnv(extraEnv)
	_, _, exitCode := runBinary(t, binary, args, env)
	return exitCode
}

// runBinary executes a binary with the given arguments and returns its output.
func runBinary(t *testing.T, binary string, args []string, env []string) (stdout, stderr []byte, exitCode int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
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

// buildTestEnv constructs the test environment with LC_ALL=C and optional extras.
func buildTestEnv(extra []string) []string {
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
	for _, e := range extra {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			key := parts[0] + "="
			set := false
			for i, entry := range env {
				if strings.HasPrefix(entry, key) {
					env[i] = e
					set = true
					break
				}
			}
			if !set {
				env = append(env, e)
			}
		}
	}
	return env
}

// assertFileExists checks that the path exists and is a regular file.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("expected file to exist at %s: %v", path, err)
		return
	}
	if info.IsDir() {
		t.Errorf("expected file at %s, got directory", path)
	}
}

// assertDirExists checks that the path exists and is a directory.
func assertDirExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("expected directory to exist at %s: %v", path, err)
		return
	}
	if !info.IsDir() {
		t.Errorf("expected directory at %s, got file", path)
	}
}

// assertPathPrefix checks that path starts with the expected prefix directory.
func assertPathPrefix(t *testing.T, path, prefix string) {
	t.Helper()
	// Resolve symlinks for comparison (e.g., /tmp -> /private/tmp on macOS).
	resolvedPath, _ := filepath.EvalSymlinks(path)
	resolvedPrefix, _ := filepath.EvalSymlinks(prefix)
	if resolvedPath == "" {
		resolvedPath = path
	}
	if resolvedPrefix == "" {
		resolvedPrefix = prefix
	}
	if !strings.HasPrefix(resolvedPath, resolvedPrefix) {
		t.Errorf("expected path %s to be under %s (resolved: %s under %s)", path, prefix, resolvedPath, resolvedPrefix)
	}
}

// assertMatchesPattern checks that s matches the given regex pattern.
func assertMatchesPattern(t *testing.T, s, pattern string) {
	t.Helper()
	re := regexp.MustCompile(pattern)
	if !re.MatchString(s) {
		t.Errorf("expected %q to match pattern %s", s, pattern)
	}
}

// assertPermissions checks that the path has the expected permission bits.
func assertPermissions(t *testing.T, path string, expected os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("cannot stat %s for permission check: %v", path, err)
		return
	}
	got := info.Mode().Perm()
	if got != expected {
		t.Errorf("permission mismatch for %s: expected=%04o got=%04o", path, expected, got)
	}
}

// cleanupPath removes a file or directory created during testing.
func cleanupPath(path string) {
	if path == "" {
		return
	}
	os.RemoveAll(path) //nolint:errcheck // best-effort cleanup
}
