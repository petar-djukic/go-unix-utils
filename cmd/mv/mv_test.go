// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/mv against GNU gmv.
// Covers prd057-mv R4.1-R4.4 (exit codes and differential testing).
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// mvTestCase describes a differential test for mv.
// setup creates test fixtures in workDir and returns args for the binary.
type mvTestCase struct {
	name  string
	setup func(t *testing.T, workDir string) []string
}

// stderrNormalizer normalizes error messages between GNU gmv and Go mv.
func stderrNormalizer(b []byte) []byte {
	binPath := regexp.MustCompile(`/[^\s:]+/g?mv|gmv`)
	tryHelp := regexp.MustCompile(
		`(?m)^Try '[^']*' for more information\.\n?`)
	noSuch := regexp.MustCompile(`(?i)no such file or directory`)
	notDir := regexp.MustCompile(`(?i)not a directory`)
	b = binPath.ReplaceAll(b, []byte("mv"))
	b = tryHelp.ReplaceAll(b, nil)
	b = noSuch.ReplaceAll(b, []byte("No such file or directory"))
	b = notDir.ReplaceAll(b, []byte("Not a directory"))
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
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, err)
		}
	}
	return binResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitCode}
}

// normalizePaths replaces absolute workDir paths with "WORK/" prefix.
func normalizePaths(b []byte, workDir string) []byte {
	return bytes.ReplaceAll(b, []byte(workDir+"/"), []byte("WORK/"))
}

// runMvDiffTest runs a single mv test against both binaries in separate dirs.
func runMvDiffTest(
	t *testing.T, goBin, refBin string, tc mvTestCase,
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
		compareMvResults(t, tc.name, refRes, goRes, refDir, goDir)
	})
}

// compareMvResults compares outputs after path normalization.
func compareMvResults(
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
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	// R4.4: differential tests covering all flag combinations.
	tests := []mvTestCase{
		{
			name: "basic_rename",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				src := writeFile(t, dir, "a.txt", []byte("hello\n"))
				return []string{src, filepath.Join(dir, "b.txt")}
			},
		},
		{
			name: "move_into_directory",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				src := writeFile(t, dir, "f.txt", []byte("data\n"))
				dstDir := makeDir(t, dir, "subdir")
				return []string{src, dstDir}
			},
		},
		{
			name: "multi_source_to_dir",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f1 := writeFile(t, dir, "x.txt", []byte("x\n"))
				f2 := writeFile(t, dir, "y.txt", []byte("y\n"))
				dstDir := makeDir(t, dir, "dest")
				return []string{f1, f2, dstDir}
			},
		},
		{
			name: "no_clobber",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				src := writeFile(t, dir, "src.txt", []byte("new\n"))
				writeFile(t, dir, "dst.txt", []byte("old\n"))
				return []string{"-n", src,
					filepath.Join(dir, "dst.txt")}
			},
		},
		{
			name: "verbose_rename",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				src := writeFile(t, dir, "v.txt", []byte("verbose\n"))
				return []string{"-v", src,
					filepath.Join(dir, "v2.txt")}
			},
		},
		{
			name: "force_overwrite",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				src := writeFile(t, dir, "fsrc.txt", []byte("force\n"))
				writeFile(t, dir, "fdst.txt", []byte("old\n"))
				return []string{"-f", src,
					filepath.Join(dir, "fdst.txt")}
			},
		},
		{
			name: "target_directory_flag",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				src := writeFile(t, dir, "t.txt", []byte("tgt\n"))
				dstDir := makeDir(t, dir, "tdir")
				return []string{"-t", dstDir, src}
			},
		},
		{
			name: "directory_move",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				srcDir := makeDir(t, dir, "moveme")
				writeFile(t, srcDir, "inner.txt", []byte("inner\n"))
				return []string{srcDir,
					filepath.Join(dir, "moved")}
			},
		},
		{
			name: "missing_source",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{
					filepath.Join(dir, "nonexistent.txt"),
					filepath.Join(dir, "out.txt"),
				}
			},
		},
		{
			name: "no_target_directory",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				src := writeFile(t, dir, "T.txt", []byte("T\n"))
				return []string{"-T", src,
					filepath.Join(dir, "Tdest")}
			},
		},
		{
			name: "missing_operand",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				return []string{}
			},
		},
		{
			name: "partial_failure_multi",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f1 := writeFile(t, dir, "ok.txt", []byte("ok\n"))
				dstDir := makeDir(t, dir, "pdest")
				return []string{
					f1, filepath.Join(dir, "nosuch.txt"), dstDir,
				}
			},
		},
		{
			name: "move_dir_into_itself",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				srcDir := makeDir(t, dir, "selfdir")
				writeFile(t, srcDir, "x.txt", []byte("x\n"))
				return []string{srcDir,
					filepath.Join(srcDir, "inside")}
			},
		},
		{
			name: "verbose_force_combined",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				src := writeFile(t, dir, "vf.txt", []byte("vf\n"))
				writeFile(t, dir, "vfdst.txt", []byte("old\n"))
				return []string{"-vf", src,
					filepath.Join(dir, "vfdst.txt")}
			},
		},
		{
			name: "target_not_a_directory",
			setup: func(t *testing.T, dir string) []string {
				t.Helper()
				f1 := writeFile(t, dir, "a.txt", []byte("a\n"))
				f2 := writeFile(t, dir, "b.txt", []byte("b\n"))
				notDir := writeFile(t, dir, "notdir.txt", []byte("x\n"))
				return []string{f1, f2, notDir}
			},
		},
	}

	for _, tc := range tests {
		runMvDiffTest(t, goBin, refBin, tc)
	}
}

// TestDiffVersionHelp tests --version and --help output.
func TestDiffVersionHelp(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gmv")
	if err != nil {
		t.Skipf("reference binary gmv not in PATH: %v", err)
	}

	versionNorm := func(b []byte) []byte {
		re := regexp.MustCompile(`(?s)^(mv) .*`)
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
	if !strings.Contains(stderr, "missing file operand") {
		t.Errorf("expected 'missing file operand' in stderr, got: %s",
			stderr)
	}
}

// TestMoveIntoSelfMessage verifies the move-into-self detection message.
// R4.2: error when attempting to move a directory into itself.
func TestMoveIntoSelfMessage(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	srcDir := makeDir(t, dir, "testdir")
	writeFile(t, srcDir, "f.txt", []byte("data\n"))
	dest := filepath.Join(srcDir, "nested")
	res := runBin(t, goBin, []string{srcDir, dest}, dir)
	if res.exitCode != 1 {
		t.Errorf("expected exit 1, got %d", res.exitCode)
	}
	stderr := string(res.stderr)
	want := fmt.Sprintf(
		"cannot move '%s' to a subdirectory of itself", srcDir)
	if !strings.Contains(stderr, want) {
		t.Errorf("expected subdirectory-of-itself error, got: %s", stderr)
	}
}
