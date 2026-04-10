// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rm against grm (GNU coreutils).
// Implements srd058 differential testing for R1.1-R1.4.
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

const refBinName = "grm"

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// writeFile creates a file with the given content in dir.
func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}

// mkDir creates a subdirectory in dir.
func mkDir(t *testing.T, dir, name string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

// programNameRe matches the program name prefix in error output.
var programNameRe = regexp.MustCompile(`(?m)^[^\s:]+:`)

// tryHelpRe matches "Try 'BINARY --help'" with any binary path.
var tryHelpRe = regexp.MustCompile(`Try '[^']+' for`)

// normalizeRm normalizes program name in stderr output.
func normalizeRm(data []byte) []byte {
	data = programNameRe.ReplaceAll(data, []byte("rm:"))
	data = tryHelpRe.ReplaceAll(data, []byte("Try 'rm --help' for"))
	return data
}

// runBin executes a binary and captures its output.
func runBin(
	t *testing.T, bin string, args []string,
	dir string, stdin []byte,
) binResult {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	err := cmd.Run()
	code := extractExitCode(t, err, bin)
	return binResult{
		stdout: stdout.Bytes(),
		stderr: stderr.Bytes(),
		exitCode: code,
	}
}

// extractExitCode gets the exit code from a command result.
func extractExitCode(t *testing.T, err error, bin string) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	t.Fatalf("failed to execute %s: %v", bin, err)
	return -1
}

// runRmDiff runs a differential test with separate temp dirs.
// Creates identical directory state for ref and go binaries,
// then compares stdout, stderr, and exit code.
func runRmDiff(
	t *testing.T, goBin, refBin string,
	setup func(t *testing.T, dir string),
	args []string,
) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()
	if setup != nil {
		setup(t, refDir)
		setup(t, goDir)
	}
	ref := runBin(t, refBin, args, refDir, nil)
	got := runBin(t, goBin, args, goDir, nil)
	compareRmResults(t, args, ref, got)
}

// compareRmResults compares stdout, stderr, and exit code.
func compareRmResults(
	t *testing.T, args []string,
	ref, got binResult,
) {
	t.Helper()
	refOut := normalizeRm(ref.stdout)
	gotOut := normalizeRm(got.stdout)
	if !bytes.Equal(refOut, gotOut) {
		t.Errorf("stdout mismatch\nargs: %v\nref:  %q\ngot:  %q",
			args, refOut, gotOut)
	}
	refErr := normalizeRm(ref.stderr)
	gotErr := normalizeRm(got.stderr)
	if !bytes.Equal(refErr, gotErr) {
		t.Errorf("stderr mismatch\nargs: %v\nref:  %q\ngot:  %q",
			args, refErr, gotErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("exit code mismatch\nargs: %v\nref: %d\ngot: %d",
			args, ref.exitCode, got.exitCode)
	}
}

// TestDiff runs differential tests comparing cmd/rm against grm.
// D2: uses exec.LookPath("grm") and t.Skip if not found.
// D4: uses testutils.BuildBinary(t, ".") to compile the Go binary.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v",
			refBinName, err)
	}
	t.Run("R1.1", func(t *testing.T) { testR1_1(t, goBin, refBin) })
	t.Run("R1.2", func(t *testing.T) { testR1_2(t, goBin, refBin) })
	t.Run("R1.3", func(t *testing.T) { testR1_3(t, goBin, refBin) })
	t.Run("R1.4", func(t *testing.T) { testR1_4(t, goBin, refBin) })
}

// testR1_1 tests basic file removal.
// R1.1: must remove each FILE argument using unlink(2).
func testR1_1(t *testing.T, goBin, refBin string) {
	t.Run("single_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "f.txt", "hello\n")
			},
			[]string{"f.txt"},
		)
	})
	t.Run("multiple_files", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "a.txt", "alpha\n")
				writeFile(t, dir, "b.txt", "beta\n")
				writeFile(t, dir, "c.txt", "gamma\n")
			},
			[]string{"a.txt", "b.txt", "c.txt"},
		)
	})
	t.Run("nonexistent_file", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"nonexistent.txt"},
		)
	})
}

// testR1_2 tests directory rejection without -r.
// R1.2: without -r, must refuse to remove a directory.
func testR1_2(t *testing.T, goBin, refBin string) {
	t.Run("directory_without_r", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "subdir")
			},
			[]string{"subdir"},
		)
	})
	t.Run("directory_with_contents", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "subdir")
				writeFile(t, dir, "subdir/file.txt", "data\n")
			},
			[]string{"subdir"},
		)
	})
}

// testR1_3 tests dot and dot-dot rejection.
// R1.3: must not remove '.' or '..'.
func testR1_3(t *testing.T, goBin, refBin string) {
	t.Run("refuse_dot", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "subdir")
			},
			[]string{"-r", "subdir/."},
		)
	})
	t.Run("refuse_dotdot", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				mkDir(t, dir, "subdir")
			},
			[]string{"-r", "subdir/.."},
		)
	})
}

// testR1_4 tests error handling and continuation.
// R1.4: must print error and continue with remaining files.
func testR1_4(t *testing.T, goBin, refBin string) {
	t.Run("continue_after_error", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "good1.txt", "ok1\n")
				writeFile(t, dir, "good2.txt", "ok2\n")
			},
			[]string{"good1.txt", "missing.txt", "good2.txt"},
		)
	})
	t.Run("mixed_file_and_dir", func(t *testing.T) {
		runRmDiff(t, goBin, refBin,
			func(t *testing.T, dir string) {
				writeFile(t, dir, "file.txt", "data\n")
				mkDir(t, dir, "subdir")
			},
			[]string{"file.txt", "subdir"},
		)
	})
	t.Run("force_nonexistent", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{"-f", "nonexistent.txt"},
		)
	})
	t.Run("no_args", func(t *testing.T) {
		runRmDiff(t, goBin, refBin, nil,
			[]string{},
		)
	})
}
