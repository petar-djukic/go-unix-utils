// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/rm against GNU grm.
// Covers prd058-rm R1.1-R1.4 (basic removal), R2.1-R2.2 (recursive/force).
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

// rmTestCase describes a differential test for rm.
// setup creates test fixtures in workDir and returns args for the binary.
type rmTestCase struct {
	name  string
	setup func(t *testing.T, workDir string) []string
}

// stderrNormalizer normalizes error messages between GNU grm and Go rm.
func stderrNormalizer(b []byte) []byte {
	binPath := regexp.MustCompile(`/[^\s:]+/g?rm|grm`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	isDir := regexp.MustCompile(`(?i)is a directory`)
	b = binPath.ReplaceAll(b, []byte("rm"))
	b = tryHelp.ReplaceAll(b, nil)
	b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
	b = isDir.ReplaceAll(b, []byte("Is a directory"))
	return b
}

// writeFile creates a file with content in dir and returns its path.
func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write test file %s: %v", p, err)
	}
	return p
}

// makeDir creates a directory in dir and returns its path.
func makeDir(t *testing.T, dir, name string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("create test dir %s: %v", p, err)
	}
	return p
}

// binResult holds the captured output of a single binary invocation.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin executes a binary with args in workDir and captures output.
func runBin(
	t *testing.T, binary string, args []string, workDir string,
) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(
		context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}
	return binResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// normalizePaths replaces absolute workDir paths with "WORK/" prefix.
func normalizePaths(b []byte, workDir string) []byte {
	return bytes.ReplaceAll(b, []byte(workDir+"/"), []byte("WORK/"))
}

// runRmDiffTest runs a single rm test against both binaries.
func runRmDiffTest(
	t *testing.T, goBin, refBin string, tc rmTestCase,
) {
	t.Helper()
	t.Run(tc.name, func(t *testing.T) {
		t.Helper()
		refDir := t.TempDir()
		goDir := t.TempDir()
		refArgs := tc.setup(t, refDir)
		goArgs := tc.setup(t, goDir)
		refRes := runBin(t, refBin, refArgs, refDir)
		goRes := runBin(t, goBin, goArgs, goDir)
		compareRmResults(t, tc.name, refRes, goRes, refDir, goDir)
	})
}

// compareRmResults compares outputs after path normalization.
func compareRmResults(
	t *testing.T, name string,
	refRes, goRes binResult,
	refDir, goDir string,
) {
	t.Helper()
	refOut := stderrNormalizer(normalizePaths(refRes.stdout, refDir))
	goOut := stderrNormalizer(normalizePaths(goRes.stdout, goDir))
	refErr := stderrNormalizer(normalizePaths(refRes.stderr, refDir))
	goErr := stderrNormalizer(normalizePaths(goRes.stderr, goDir))
	if !bytes.Equal(refOut, goOut) ||
		!bytes.Equal(refErr, goErr) ||
		refRes.exitCode != goRes.exitCode {
		t.Errorf("divergence in %s\n"+
			"  ref stdout: %q\n  go  stdout: %q\n"+
			"  ref stderr: %q\n  go  stderr: %q\n"+
			"  ref exit: %d\n  go  exit: %d",
			name, refOut, goOut, refErr, goErr,
			refRes.exitCode, goRes.exitCode)
	}
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	tests := []rmTestCase{
		{
			// R1.1: remove a single regular file.
			name: "single_file",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f := writeFile(t, dir, "a.txt", []byte("hello\n"))
				return []string{f}
			},
		},
		{
			// R1.1: remove multiple files.
			name: "multi_file",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f1 := writeFile(t, dir, "x.txt", []byte("x\n"))
				f2 := writeFile(t, dir, "y.txt", []byte("y\n"))
				return []string{f1, f2}
			},
		},
		{
			// R1.2: error when removing directory without -r.
			name: "dir_without_r",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "subdir")
				return []string{d}
			},
		},
		{
			// R2.2: -f silently ignores nonexistent files.
			name: "force_nonexistent",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{"-f",
					filepath.Join(dir, "nosuch.txt")}
			},
		},
		{
			// R2.2: without -f, nonexistent file is an error.
			name: "nonexistent_error",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{
					filepath.Join(dir, "nosuch.txt")}
			},
		},
		{
			// R3.3: -v prints removed file names.
			name: "verbose_remove",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f := writeFile(t, dir, "v.txt", []byte("v\n"))
				return []string{"-v", f}
			},
		},
		{
			// R2.1: -r removes directories recursively.
			name: "recursive_dir",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "tree")
				writeFile(t, d, "inner.txt", []byte("data\n"))
				makeDir(t, d, "nested")
				writeFile(t, filepath.Join(d, "nested"),
					"deep.txt", []byte("deep\n"))
				return []string{"-r", d}
			},
		},
		{
			// R2.1 + R2.2: -rf combined flags.
			name: "rf_combined",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "rfdir")
				writeFile(t, d, "f.txt", []byte("rf\n"))
				return []string{"-rf", d}
			},
		},
		{
			// R2.4: -d removes empty directories.
			name: "d_empty_dir",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "emptydir")
				return []string{"-d", d}
			},
		},
		{
			// R3.3: -rv verbose recursive removal.
			name: "recursive_verbose",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				d := makeDir(t, dir, "rvdir")
				writeFile(t, d, "a.txt", []byte("a\n"))
				return []string{"-rv", d}
			},
		},
		{
			// R1.4: partial failure continues with remaining.
			name: "partial_failure",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f1 := writeFile(t, dir, "ok.txt", []byte("ok\n"))
				return []string{f1,
					filepath.Join(dir, "nope.txt")}
			},
		},
		{
			// No operands without -f.
			name: "missing_operand",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{}
			},
		},
		{
			// -f with no operands exits 0.
			name: "force_no_operands",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{"-f"}
			},
		},
	}

	for _, tc := range tests {
		runRmDiffTest(t, goBin, refBin, tc)
	}
}

// TestDiffVersionHelp tests --version and --help output.
func TestDiffVersionHelp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("grm")
	if err != nil {
		t.Skipf("reference binary grm not in PATH: %v", err)
	}

	versionNorm := func(b []byte) []byte {
		re := regexp.MustCompile(`(?s)^(rm) .*`)
		return re.ReplaceAll(b, []byte("$1 (version)\n"))
	}
	helpNorm := func(b []byte) []byte {
		if len(b) > 0 {
			return []byte("HELP_OUTPUT\n")
		}
		return b
	}

	tests := []testutils.DiffTest{
		{
			Name:      "version_flag",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{versionNorm},
		},
		{
			Name:      "help_flag",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{helpNorm},
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestMissingOperandMessage verifies the missing-operand error format.
func TestMissingOperandMessage(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	res := runBin(t, goBin, []string{}, t.TempDir())
	if res.exitCode != 1 {
		t.Errorf("expected exit 1, got %d", res.exitCode)
	}
	stderr := string(res.stderr)
	if !strings.Contains(stderr, "missing operand") {
		t.Errorf(
			"expected 'missing operand' in stderr, got: %s", stderr)
	}
}

// TestDotDotDotRefusal verifies . and .. are refused.
// R1.3: prevent accidental directory tree destruction.
func TestDotDotDotRefusal(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	for _, path := range []string{".", "..", "/tmp/."} {
		res := runBin(t, goBin, []string{path}, t.TempDir())
		if res.exitCode != 1 {
			t.Errorf("rm %s: expected exit 1, got %d",
				path, res.exitCode)
		}
		stderr := string(res.stderr)
		if !strings.Contains(stderr, "refusing to remove") {
			t.Errorf("rm %s: expected refusing message, got: %s",
				path, stderr)
		}
	}
}
