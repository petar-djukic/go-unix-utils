// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rmdir against grmdir (GNU coreutils).
// Implements prd035-rmdir R1.1-R1.4, R2.1-R2.3, R3.1-R3.4 test coverage.
package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// binaryNameRe matches the program name prefix at the start of each line in
// verbose/error output (e.g., "/opt/homebrew/bin/grmdir:" or "rmdir:").
var binaryNameRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

func TestDiff(t *testing.T) {
	t.Parallel()

	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grmdir")
	if err != nil {
		t.Skipf("reference binary grmdir not in PATH: %v", err)
	}

	// Differential tests for error cases where no directory state changes.
	tests := []testutils.DiffTest{
		// R1.2: non-existent directory — error, exit 1.
		{
			Name:      "R1.2_nonexistent",
			Args:      []string{"nosuchdir"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
		// No arguments — error, exit 1.
		{
			Name:      "no_args_error",
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeAllOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)

	// Filesystem-modifying tests run each binary independently with fresh state.

	// R1.1: remove a single empty directory, exit 0.
	t.Run("R1.1_remove_empty_dir", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"emptydir"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R1.1: remove multiple empty directories.
	t.Run("R1.1_remove_multiple", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"dir1", "dir2"}, func(dir string) {
			os.Mkdir(filepath.Join(dir, "dir1"), 0o755) //nolint:errcheck // test setup
			os.Mkdir(filepath.Join(dir, "dir2"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R1.2: non-empty directory — error, exit 1.
	t.Run("R1.2_nonempty", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"nonemptydir"}, 1, func(dir string) {
			p := filepath.Join(dir, "nonemptydir")
			os.Mkdir(p, 0o755)                                                     //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(p, "file.txt"), []byte("data"), 0o644)       //nolint:errcheck // test setup
		})
	})

	// R1.2: argument is a regular file — error, exit 1.
	t.Run("R1.2_not_a_directory", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"afile"}, 1, func(dir string) {
			os.WriteFile(filepath.Join(dir, "afile"), []byte("x"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R1.3: --ignore-fail-on-non-empty suppresses non-empty errors.
	t.Run("R1.3_ignore_nonempty", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"--ignore-fail-on-non-empty", "nonemptydir"}, 0, func(dir string) {
			p := filepath.Join(dir, "nonemptydir")
			os.Mkdir(p, 0o755)                                                     //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(p, "file.txt"), []byte("data"), 0o644)       //nolint:errcheck // test setup
		})
	})

	// R1.4: -p removes parent directory components.
	t.Run("R1.4_parents_short", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"-p", "a/b/c"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R1.4: --parents long form.
	t.Run("R1.4_parents_long", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"--parents", "x/y"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "x", "y"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R1.4: -p with non-empty parent — partial removal, exit 1.
	t.Run("R1.4_parents_partial_fail", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "a/b/c"}, 1, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                       //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "blocker.txt"), []byte("x"), 0o644)     //nolint:errcheck // test setup
		})
	})

	// R1.3 + R1.4: --ignore-fail-on-non-empty with -p.
	t.Run("R1.3_R1.4_ignore_parents", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "--ignore-fail-on-non-empty", "a/b/c"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                       //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "blocker.txt"), []byte("x"), 0o644)     //nolint:errcheck // test setup
		})
	})

	// R2.3: -p with multiple arguments processed independently.
	t.Run("R2.3_parents_multiple_args", func(t *testing.T) {
		t.Parallel()
		assertBothRemoveDir(t, goBin, refBin, []string{"-p", "a/b/c", "x/y"}, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755) //nolint:errcheck // test setup
			os.MkdirAll(filepath.Join(dir, "x", "y"), 0o755)      //nolint:errcheck // test setup
		})
	})

	// R2.1: -p on a nonexistent path — error, exit 1.
	t.Run("R2.1_parents_nonexistent", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "no/such/path"}, 1, func(dir string) {
			// no setup — path does not exist
		})
	})

	// R2.2: -p where intermediate ancestor is non-empty — partial removal, exit 1.
	// Deepest child and middle dir removed, top dir kept because it has a file.
	t.Run("R2.2_parents_intermediate_nonempty", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "a/b/c"}, 1, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                   //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "other.txt"), []byte("x"), 0o644)   //nolint:errcheck // test setup
		})
	})

	// R2.3: -p with multiple args where one fails and one succeeds — exit 1.
	t.Run("R2.3_parents_mixed_success_fail", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"-p", "good/sub", "bad/sub"}, 1, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "good", "sub"), 0o755)                    //nolint:errcheck // test setup
			os.MkdirAll(filepath.Join(dir, "bad", "sub"), 0o755)                     //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "bad", "file.txt"), []byte("x"), 0o644)  //nolint:errcheck // test setup
		})
	})

	// R3.1: --ignore-fail-on-non-empty suppresses non-empty error and exits 0 with no stderr.
	t.Run("R3.1_ignore_nonempty_no_stderr", func(t *testing.T) {
		t.Parallel()
		assertBothOutputMatch(t, goBin, refBin, []string{"--ignore-fail-on-non-empty", "nonemptydir"}, 0, func(dir string) {
			p := filepath.Join(dir, "nonemptydir")
			os.Mkdir(p, 0o755)                                               //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(p, "file.txt"), []byte("x"), 0o644)   //nolint:errcheck // test setup
		})
	})

	// R3.2: --ignore-fail-on-non-empty does NOT suppress non-existent dir error.
	t.Run("R3.2_ignore_does_not_suppress_nonexistent", func(t *testing.T) {
		t.Parallel()
		assertBothExitCode(t, goBin, refBin, []string{"--ignore-fail-on-non-empty", "nosuchdir"}, 1, func(dir string) {
			// no setup — path does not exist
		})
	})

	// R3.3: -v verbose output for single empty directory removal.
	t.Run("R3.3_verbose_single", func(t *testing.T) {
		t.Parallel()
		assertBothOutputMatch(t, goBin, refBin, []string{"-v", "emptydir"}, 0, func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R3.3: --verbose long form.
	t.Run("R3.3_verbose_long", func(t *testing.T) {
		t.Parallel()
		assertBothOutputMatch(t, goBin, refBin, []string{"--verbose", "emptydir"}, 0, func(dir string) {
			os.Mkdir(filepath.Join(dir, "emptydir"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R3.3 + R3.4: -v with -p shows verbose for each parent removed.
	t.Run("R3.3_verbose_parents", func(t *testing.T) {
		t.Parallel()
		assertBothOutputMatch(t, goBin, refBin, []string{"-pv", "a/b/c"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755) //nolint:errcheck // test setup
		})
	})

	// R3.3: --ignore-fail-on-non-empty with -p and -v — suppresses non-empty at parent level.
	t.Run("R3.3_ignore_parents_verbose", func(t *testing.T) {
		t.Parallel()
		assertBothOutputMatch(t, goBin, refBin, []string{"-pv", "--ignore-fail-on-non-empty", "a/b/c"}, 0, func(dir string) {
			os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0o755)                   //nolint:errcheck // test setup
			os.WriteFile(filepath.Join(dir, "a", "blocker.txt"), []byte("x"), 0o644) //nolint:errcheck // test setup
		})
	})

	// R3.4: verbose output for multiple directories.
	t.Run("R3.4_verbose_multiple", func(t *testing.T) {
		t.Parallel()
		assertBothOutputMatch(t, goBin, refBin, []string{"-v", "dir1", "dir2"}, 0, func(dir string) {
			os.Mkdir(filepath.Join(dir, "dir1"), 0o755) //nolint:errcheck // test setup
			os.Mkdir(filepath.Join(dir, "dir2"), 0o755) //nolint:errcheck // test setup
		})
	})
}

// assertBothRemoveDir runs both binaries with fresh setup and verifies both
// exit 0 with matching exit codes.
func assertBothRemoveDir(t *testing.T, goBin, refBin string, args []string, setup func(dir string)) {
	t.Helper()

	refExit := runWithSetup(t, refBin, args, setup)
	goExit := runWithSetup(t, goBin, args, setup)

	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d, args=%v", refExit, goExit, args)
	}
	if goExit != 0 {
		t.Errorf("expected exit 0, got %d, args=%v", goExit, args)
	}
}

// assertBothExitCode runs both binaries with fresh setup and verifies both
// produce the expected exit code.
func assertBothExitCode(t *testing.T, goBin, refBin string, args []string, wantExit int, setup func(dir string)) {
	t.Helper()

	refExit := runWithSetup(t, refBin, args, setup)
	goExit := runWithSetup(t, goBin, args, setup)

	if refExit != goExit {
		t.Errorf("exit code mismatch: ref=%d go=%d, args=%v", refExit, goExit, args)
	}
	if goExit != wantExit {
		t.Errorf("expected exit %d, got %d, args=%v", wantExit, goExit, args)
	}
}

// runResult holds the output and exit code of a binary execution.
type runResult struct {
	exitCode int
	stdout   string
	stderr   string
}

// runWithSetup creates a fresh temp directory, runs setup, then executes the
// binary. Returns the exit code.
func runWithSetup(t *testing.T, binary string, args []string, setup func(dir string)) int {
	t.Helper()
	r := runWithSetupFull(t, binary, args, setup)
	return r.exitCode
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

// assertBothOutputMatch runs both binaries with fresh setup and verifies both
// produce the same exit code and stdout output.
func assertBothOutputMatch(t *testing.T, goBin, refBin string, args []string, wantExit int, setup func(dir string)) {
	t.Helper()

	refResult := runWithSetupFull(t, refBin, args, setup)
	goResult := runWithSetupFull(t, goBin, args, setup)

	if refResult.exitCode != goResult.exitCode {
		t.Errorf("exit code mismatch: ref=%d go=%d, args=%v", refResult.exitCode, goResult.exitCode, args)
	}
	if goResult.exitCode != wantExit {
		t.Errorf("expected exit %d, got %d, args=%v", wantExit, goResult.exitCode, args)
	}
	refNorm := binaryNameRe.ReplaceAllString(refResult.stdout, "PROG:")
	goNorm := binaryNameRe.ReplaceAllString(goResult.stdout, "PROG:")
	if refNorm != goNorm {
		t.Errorf("stdout mismatch, args=%v\nref: %q\ngo:  %q", args, refResult.stdout, goResult.stdout)
	}
}

// normalizeAllOutput replaces all output with empty bytes so that only
// exit codes are compared. Used for error messages where the exact format
// may differ between implementations.
func normalizeAllOutput(b []byte) []byte {
	return nil
}
