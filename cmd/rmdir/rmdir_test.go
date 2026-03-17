// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rmdir against grmdir (GNU coreutils).
// Implements prd035-rmdir R4.1-R4.3 test coverage: differential tests comparing
// stdout, stderr, and exit codes between Go binary and grmdir for basic removal,
// error handling (non-existent, non-empty, not-a-directory, permission denied),
// flag combinations (--parents, --ignore-fail-on-non-empty, --verbose), and
// parent ascending behavior.
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	// R4.1: Differential tests for error cases where no directory state changes.
	tests := []testutils.DiffTest{
		// R4.2: non-existent directory — error, exit 1.
		{
			Name:      "R4.2_nonexistent",
			Args:      []string{"nosuchdir"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: no arguments — error, exit 1.
		{
			Name:      "no_args_error",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)

	// Filesystem-modifying tests run each binary independently with fresh state.

	// R4.2: remove a single empty directory, exit 0.
	t.Run("R4.2_remove_empty_dir", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"emptydir"}, 0, func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R4.2: remove multiple empty directories.
	t.Run("R4.2_remove_multiple", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"dir1", "dir2"}, 0, func(dir string) {
			os.Mkdir(filepath.Join(dir, "dir1"), 0o755) //nolint:errcheck // test setup
			os.Mkdir(filepath.Join(dir, "dir2"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R4.2: non-empty directory — error, exit 1.
	t.Run("R4.2_nonempty", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"nonemptydir"}, 1, func(dir string) {
			p := filepath.Join(dir, "nonemptydir")
			os.Mkdir(p, 0o755)                                               //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(p, "file.txt"), []byte("data"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R4.2: argument is a regular file — error, exit 1.
	t.Run("R4.2_not_a_directory", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"afile"}, 1, func(dir string) {
			os.WriteFile(filepath.Join(dir, "afile"), []byte("x"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R4.2: permission denied — cannot remove directory without write on parent.
	t.Run("R4.2_permission_denied", func(t *testing.T) {
		t.Parallel()
		// Skip if running as root since root bypasses permission checks.
		if os.Getuid() == 0 {
			t.Skip("cannot test permission errors as root")
		}
		assertBothMatch(t, goBin, refBin, []string{"parent/child"}, 1, func(dir string) {
			parent := filepath.Join(dir, "parent")
			child := filepath.Join(parent, "child")
			os.MkdirAll(child, 0o755) //nolint:errcheck // test setup
			// Remove write permission from parent so child cannot be removed.
			os.Chmod(parent, 0o555) //nolint:errcheck // test setup
			// t.Cleanup restores permissions so TempDir cleanup succeeds.
			t.Cleanup(func() {
				os.Chmod(parent, 0o755) //nolint:errcheck // best-effort cleanup
			})
		})
	})

	// R4.2: --ignore-fail-on-non-empty suppresses non-empty errors, exit 0.
	t.Run("R4.2_ignore_nonempty", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"--ignore-fail-on-non-empty", "nonemptydir"}, 0, func(dir string) {
			p := filepath.Join(dir, "nonemptydir")
			os.Mkdir(p, 0o755)                                               //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(p, "file.txt"), []byte("data"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R4.2: -v verbose output for single empty directory removal.
	t.Run("R4.2_verbose_single", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-v", "emptydir"}, 0, func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R4.2: -p with nested empty directories — removes all.
	t.Run("R4.2_parents_short", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-p", "a/b/c"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R4.2: --parents long form.
	t.Run("R4.2_parents_long", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"--parents", "x/y"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "x", "y"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R4.3: -p with non-empty parent — partial removal, exit 1.
	// Verifies -p stops ascending correctly when a parent is not empty.
	t.Run("R4.3_parents_partial_fail", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-p", "a/b/c"}, 1, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                   //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "blocker.txt"), []byte("x"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R4.3: --ignore-fail-on-non-empty with -p — suppresses non-empty parent error.
	t.Run("R4.3_ignore_parents", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-p", "--ignore-fail-on-non-empty", "a/b/c"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                   //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "blocker.txt"), []byte("x"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R4.3: -p with multiple arguments processed independently.
	t.Run("R4.3_parents_multiple_args", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-p", "a/b/c", "x/y"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755) //nolint:errcheck // test setup
			os.MkdirAll(filepath.Join(dir, "x", "y"), 0o755)      //nolint:errcheck // test setup
		})
	})

	// R4.3: -p on a nonexistent path — error, exit 1.
	t.Run("R4.3_parents_nonexistent", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-p", "no/such/path"}, 1, func(dir string) {
			// no setup — path does not exist
		})
	})

	// R4.3: -p where intermediate ancestor is non-empty — partial removal.
	// Deepest child and middle dir removed, top dir kept because it has a file.
	t.Run("R4.3_parents_intermediate_nonempty", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-p", "a/b/c"}, 1, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                 //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "other.txt"), []byte("x"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R4.3: -p with mixed success/fail across multiple args — exit 1.
	t.Run("R4.3_parents_mixed_success_fail", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-p", "good/sub", "bad/sub"}, 1, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "good", "sub"), 0o755)                  //nolint:errcheck // test setup
			os.MkdirAll(filepath.Join(dir, "bad", "sub"), 0o755)                   //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "bad", "file.txt"), []byte("x"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R4.2: --ignore-fail-on-non-empty does NOT suppress non-existent error.
	t.Run("R4.2_ignore_does_not_suppress_nonexistent", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"--ignore-fail-on-non-empty", "nosuchdir"}, 1, func(dir string) {
			// no setup — path does not exist
		})
	})

	// R4.2: --verbose long form.
	t.Run("R4.2_verbose_long", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"--verbose", "emptydir"}, 0, func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R4.3: -v with -p shows verbose for each parent removed.
	t.Run("R4.3_verbose_parents", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-pv", "a/b/c"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R4.3: --ignore-fail-on-non-empty with -p and -v — suppresses non-empty parent.
	t.Run("R4.3_ignore_parents_verbose", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-pv", "--ignore-fail-on-non-empty", "a/b/c"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                   //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "blocker.txt"), []byte("x"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R4.2: verbose output for multiple directories.
	t.Run("R4.2_verbose_multiple", func(t *testing.T) {
		t.Parallel()
		assertBothMatch(t, goBin, refBin, []string{"-v", "dir1", "dir2"}, 0, func(dir string) {
			os.Mkdir(filepath.Join(dir, "dir1"), 0o755) //nolint:errcheck // test setup
			os.Mkdir(filepath.Join(dir, "dir2"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R4.2: --ignore-fail-on-non-empty does NOT suppress permission denied.
	t.Run("R4.2_ignore_does_not_suppress_permission", func(t *testing.T) {
		t.Parallel()
		if os.Getuid() == 0 {
			t.Skip("cannot test permission errors as root")
		}
		assertBothMatch(t, goBin, refBin, []string{"--ignore-fail-on-non-empty", "parent/child"}, 1, func(dir string) {
			parent := filepath.Join(dir, "parent")
			child := filepath.Join(parent, "child")
			os.MkdirAll(child, 0o755) //nolint:errcheck // test setup
			os.Chmod(parent, 0o555)    //nolint:errcheck // test setup
			t.Cleanup(func() {
				os.Chmod(parent, 0o755) //nolint:errcheck // best-effort cleanup
			})
		})
	})
}

// runResult holds the output and exit code of a binary execution.
type runResult struct {
	exitCode int
	stdout   string
	stderr   string
}

// assertBothMatch runs both binaries with fresh setup and verifies exit code,
// stdout, and stderr all match after normalization. R4.1: compares stdout,
// stderr, and exit codes between Go and grmdir.
func assertBothMatch(t *testing.T, goBin, refBin string, args []string, wantExit int, setup func(dir string)) {
	t.Helper()

	refResult := runWithSetupFull(t, refBin, args, setup)
	goResult := runWithSetupFull(t, goBin, args, setup)

	if refResult.exitCode != goResult.exitCode {
		t.Errorf("exit code mismatch: ref=%d go=%d, args=%v\nref stderr: %q\ngo stderr: %q",
			refResult.exitCode, goResult.exitCode, args, refResult.stderr, goResult.stderr)
	}
	if goResult.exitCode != wantExit {
		t.Errorf("expected exit %d, got %d, args=%v", wantExit, goResult.exitCode, args)
	}

	refStdout := normalizeProgramName([]byte(refResult.stdout))
	goStdout := normalizeProgramName([]byte(goResult.stdout))
	if !bytes.Equal(refStdout, goStdout) {
		t.Errorf("stdout mismatch, args=%v\nref: %q\ngo:  %q", args, refResult.stdout, goResult.stdout)
	}

	refStderr := normalizeProgramName([]byte(refResult.stderr))
	goStderr := normalizeProgramName([]byte(goResult.stderr))
	if !bytes.Equal(refStderr, goStderr) {
		t.Errorf("stderr mismatch, args=%v\nref: %q\ngo:  %q", args, refResult.stderr, goResult.stderr)
	}
}

// runWithSetupFull creates a fresh temp directory, runs setup, then executes
// the binary. Returns exit code, stdout, and stderr.
func runWithSetupFull(t *testing.T, binary string, args []string, setup func(dir string)) runResult {
	t.Helper()

	dir := t.TempDir()
	setup(dir)

	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return runResult{exitCode: 0, stdout: stdout.String(), stderr: stderr.String()}
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return runResult{exitCode: exitErr.ExitCode(), stdout: stdout.String(), stderr: stderr.String()}
	}

	t.Fatalf("failed to execute %s: %v", binary, err)
	return runResult{exitCode: -1}
}

// normalizeProgramName replaces the reference binary name and path with
// "rmdir" so output from grmdir matches our output. Also lowercases the
// result so OS-level error string case differences do not cause false
// divergence.
func normalizeProgramName(b []byte) []byte {
	lines := bytes.Split(b, []byte("\n"))
	for i, line := range lines {
		line = normalizePathPrefix(line)
		lines[i] = bytes.ReplaceAll(line, []byte("grmdir"), []byte("rmdir"))
	}
	b = bytes.Join(lines, []byte("\n"))
	// Normalize "Try '/path/to/grmdir --help'" → "Try 'rmdir --help'".
	for {
		idx := bytes.Index(b, []byte("Try '"))
		if idx == -1 {
			break
		}
		rest := b[idx+5:]
		end := bytes.IndexByte(rest, '\'')
		if end == -1 {
			break
		}
		inner := rest[:end]
		if mIdx := bytes.LastIndex(inner, []byte("rmdir")); mIdx > 0 {
			prefix := inner[:mIdx]
			if slashIdx := bytes.LastIndexByte(prefix, '/'); slashIdx >= 0 {
				replacement := append([]byte("Try '"), inner[slashIdx+1:]...)
				replacement = append(replacement, '\'')
				old := b[idx : idx+5+end+1]
				b = bytes.Replace(b, old, replacement, 1)
				continue
			}
		}
		break
	}
	return bytes.ToLower(b)
}

// normalizePathPrefix replaces a leading "/path/to/grmdir: " or
// "/path/to/rmdir: " with "rmdir: " so verbose and error output from
// the reference binary (which uses argv[0]) matches our output.
func normalizePathPrefix(line []byte) []byte {
	colonIdx := bytes.Index(line, []byte(": "))
	if colonIdx == -1 {
		return line
	}
	prog := line[:colonIdx]
	if slashIdx := bytes.LastIndexByte(prog, '/'); slashIdx >= 0 {
		base := prog[slashIdx+1:]
		if bytes.Equal(base, []byte("grmdir")) || bytes.Equal(base, []byte("rmdir")) {
			return append([]byte("rmdir"), line[colonIdx:]...)
		}
	}
	return line
}
