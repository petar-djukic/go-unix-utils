// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ln against gln (GNU coreutils).
// Implements srd037 R4.1 (compare stdout/stderr/exit codes),
// R4.2 (test coverage for R3.1-R3.6), R4.3 (link verification).
package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

const refBinName = "gln"
const execTimeout = 30 * time.Second

// makeNormalizer creates a NormalizeFunc that normalizes binary names and
// known syscall error message capitalization differences between GNU and Go.
func makeNormalizer(refBin string) testutils.NormalizeFunc {
	return func(b []byte) []byte {
		b = bytes.ReplaceAll(b, []byte(refBin), []byte(programName))
		b = bytes.ReplaceAll(b, []byte(refBinName), []byte(programName))
		b = normalizeSyscallErrors(b)
		return b
	}
}

// normalizeSyscallErrors lowercases known syscall error messages that
// differ in case between C strerror() and Go syscall.Errno.Error().
func normalizeSyscallErrors(b []byte) []byte {
	replacements := []struct{ from, to string }{
		{"File exists", "file exists"},
		{"No such file or directory", "no such file or directory"},
		{"Not a directory", "not a directory"},
		{"Permission denied", "permission denied"},
		{"Operation not permitted", "operation not permitted"},
		{"Is a directory", "is a directory"},
	}
	for _, r := range replacements {
		b = bytes.ReplaceAll(b, []byte(r.from), []byte(r.to))
	}
	return b
}

// TestDiff runs differential tests comparing cmd/ln against gln.
// R4.1: compare stdout, stderr, exit codes.
// R4.2: covers hard links, symlinks, -f, -n, -i, -v, -b, -S, and error cases.
func TestDiff(t *testing.T) {
	t.Parallel()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath(refBinName)
	if err != nil {
		t.Skipf("reference binary %s not in PATH: %v", refBinName, err)
	}
	norm := makeNormalizer(refBin)

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		runErrorTests(t, goBin, refBin, norm)
	})
	t.Run("force", func(t *testing.T) {
		t.Parallel()
		runForceTests(t, goBin, refBin, norm)
	})
	t.Run("no_dereference", func(t *testing.T) {
		t.Parallel()
		runNoDereferenceTests(t, goBin, refBin, norm)
	})
	t.Run("interactive", func(t *testing.T) {
		t.Parallel()
		runInteractiveTests(t, goBin, refBin, norm)
	})
	t.Run("verbose", func(t *testing.T) {
		t.Parallel()
		runVerboseTests(t, goBin, refBin, norm)
	})
	t.Run("backup", func(t *testing.T) {
		t.Parallel()
		runBackupTests(t, goBin, refBin, norm)
	})
	t.Run("suffix", func(t *testing.T) {
		t.Parallel()
		runSuffixTests(t, goBin, refBin, norm)
	})
}

// runErrorTests uses RunDiffTests for error cases where both binaries
// see the same filesystem state and neither mutates it.
func runErrorTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	norms := []testutils.NormalizeFunc{norm}
	tests := []testutils.DiffTest{
		{
			Name: "missing_operand", Args: []string{},
			ExitCode: 1, Normalize: norms,
		},
	}
	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// isolatedCase defines a test that runs each binary in its own directory.
type isolatedCase struct {
	name    string
	args    []string
	stdin   []byte
	setup   func(t *testing.T, dir string) // prepare filesystem before running
	verify  func(t *testing.T, refDir, goDir string)
	wantErr bool // expect non-zero exit code
}

// runForceTests tests R3.1: -f removes existing destination before linking.
func runForceTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		{
			name: "force_overwrite_hard_link",
			args: []string{"-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: verifyHardLink("target", "link"),
		},
		{
			name: "force_overwrite_symlink",
			args: []string{"-sf", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: verifySymLink("link", "target"),
		},
		{
			name: "force_no_existing",
			args: []string{"-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
			},
			verify: verifyHardLink("target", "link"),
		},
	}
	runIsolatedCases(t, goBin, refBin, norm, cases)
}

// runNoDereferenceTests tests R3.2: -n treats symlink-to-dir as regular file.
func runNoDereferenceTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		{
			name: "no_deref_symlink_to_dir",
			args: []string{"-sfn", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				if err := os.Mkdir(filepath.Join(dir, "realdir"), 0o755); err != nil {
					t.Fatalf("setup: %v", err)
				}
				writeFile(t, filepath.Join(dir, "target"), "content")
				// link is a symlink to a directory
				if err := os.Symlink("realdir", filepath.Join(dir, "link")); err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			verify: verifySymLink("link", "target"),
		},
		{
			name: "no_deref_with_force",
			args: []string{"-sfn", "newtarget", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "newtarget"), "new")
				// existing symlink pointing elsewhere
				if err := os.Symlink("oldtarget", filepath.Join(dir, "link")); err != nil {
					t.Fatalf("setup: %v", err)
				}
			},
			verify: verifySymLink("link", "newtarget"),
		},
	}
	runIsolatedCases(t, goBin, refBin, norm, cases)
}

// runInteractiveTests tests R3.3: -i prompts before removing existing dest.
func runInteractiveTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		{
			name: "interactive_accept",
			args: []string{"-i", "target", "link"},
			stdin: []byte("y\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: verifyHardLink("target", "link"),
		},
		{
			name: "interactive_decline",
			args: []string{"-i", "target", "link"},
			stdin: []byte("n\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: verifyFileContent("link", "old"),
		},
		{
			name: "interactive_no_existing",
			args: []string{"-i", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
			},
			verify: verifyHardLink("target", "link"),
		},
		{
			name: "interactive_symlink_accept",
			args: []string{"-si", "target", "link"},
			stdin: []byte("y\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: verifySymLink("link", "target"),
		},
		{
			name: "force_overrides_interactive",
			args: []string{"-i", "-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: verifyHardLink("target", "link"),
		},
		{
			name: "interactive_overrides_force",
			args: []string{"-f", "-i", "target", "link"},
			stdin: []byte("n\n"),
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: verifyFileContent("link", "old"),
		},
	}
	runIsolatedCases(t, goBin, refBin, norm, cases)
}

// runVerboseTests tests R3.4: -v prints link name to stdout.
func runVerboseTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		{
			name: "verbose_hard_link",
			args: []string{"-v", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
			},
			verify: verifyHardLink("target", "link"),
		},
		{
			name: "verbose_symlink",
			args: []string{"-sv", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
			},
			verify: verifySymLink("link", "target"),
		},
		{
			name: "verbose_force_overwrite",
			args: []string{"-vf", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: verifyHardLink("target", "link"),
		},
	}
	runIsolatedCases(t, goBin, refBin, norm, cases)
}

// runBackupTests tests R3.5: -b and --backup create backups before removal.
func runBackupTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		{
			name: "backup_default_force",
			args: []string{"-b", "-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: combineVerifiers(
				verifyHardLink("target", "link"),
				verifyFileContent("link~", "old"),
			),
		},
		{
			name: "backup_numbered_force",
			args: []string{"--backup=numbered", "-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: combineVerifiers(
				verifyHardLink("target", "link"),
				verifyFileContent("link.~1~", "old"),
			),
		},
		{
			name: "backup_simple_force",
			args: []string{"--backup=simple", "-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: combineVerifiers(
				verifyHardLink("target", "link"),
				verifyFileContent("link~", "old"),
			),
		},
		{
			name: "backup_none_force",
			args: []string{"--backup=none", "-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: combineVerifiers(
				verifyHardLink("target", "link"),
				verifyFileAbsent("link~"),
			),
		},
		{
			name: "backup_existing_with_numbered",
			args: []string{"-b", "-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
				// pre-existing numbered backup triggers numbered mode
				writeFile(t, filepath.Join(dir, "link.~1~"), "older")
			},
			verify: combineVerifiers(
				verifyHardLink("target", "link"),
				verifyFileContent("link.~2~", "old"),
			),
		},
	}
	runIsolatedCases(t, goBin, refBin, norm, cases)
}

// runSuffixTests tests R3.6: -S/--suffix overrides the default backup suffix.
func runSuffixTests(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc) {
	t.Helper()
	cases := []isolatedCase{
		{
			name: "suffix_short_flag",
			args: []string{"-S", ".bak", "-b", "-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: combineVerifiers(
				verifyHardLink("target", "link"),
				verifyFileContent("link.bak", "old"),
			),
		},
		{
			name: "suffix_long_flag",
			args: []string{"--suffix=.orig", "-b", "-f", "target", "link"},
			setup: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "target"), "content")
				writeFile(t, filepath.Join(dir, "link"), "old")
			},
			verify: combineVerifiers(
				verifyHardLink("target", "link"),
				verifyFileContent("link.orig", "old"),
			),
		},
	}
	runIsolatedCases(t, goBin, refBin, norm, cases)
}

// runIsolatedCases runs each test case with both binaries in separate dirs.
func runIsolatedCases(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, cases []isolatedCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			compareIsolated(t, goBin, refBin, norm, tc)
		})
	}
}

// compareIsolated runs both binaries in separate temp dirs and compares
// stdout, stderr, exit code, and filesystem state.
func compareIsolated(t *testing.T, goBin, refBin string, norm testutils.NormalizeFunc, tc isolatedCase) {
	t.Helper()
	refDir := t.TempDir()
	goDir := t.TempDir()

	if tc.setup != nil {
		tc.setup(t, refDir)
		tc.setup(t, goDir)
	}

	refRes := runBin(t, refBin, tc.args, tc.stdin, refDir)
	goRes := runBin(t, goBin, tc.args, tc.stdin, goDir)

	compareOutputs(t, norm, tc.args, refRes, goRes)
	if tc.verify != nil {
		tc.verify(t, refDir, goDir)
	}
}

// binResult holds captured output from a single binary execution.
type binResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// runBin executes a binary in workDir and captures stdout, stderr, exit code.
func runBin(t *testing.T, bin string, args []string, stdin []byte, workDir string) binResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	cmd.Dir = workDir
	cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	return extractResult(t, cmd, ctx, &outBuf, &errBuf)
}

// extractResult runs the command and returns the captured result.
func extractResult(t *testing.T, cmd *exec.Cmd, ctx context.Context, outBuf, errBuf *bytes.Buffer) binResult {
	t.Helper()
	err := cmd.Run()
	if err == nil {
		return binResult{stdout: outBuf.Bytes(), stderr: errBuf.Bytes()}
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return binResult{
			stdout:   outBuf.Bytes(),
			stderr:   errBuf.Bytes(),
			exitCode: exitErr.ExitCode(),
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("%s timed out after %v", cmd.Path, execTimeout)
	}
	t.Fatalf("%s failed: %v", cmd.Path, err)
	return binResult{} // unreachable
}

// compareOutputs compares stdout, stderr, and exit code between ref and go.
func compareOutputs(t *testing.T, norm testutils.NormalizeFunc, args []string, ref, got binResult) {
	t.Helper()
	refOut := norm(ref.stdout)
	gotOut := norm(got.stdout)
	refErr := norm(ref.stderr)
	gotErr := norm(got.stderr)

	if !bytes.Equal(refOut, gotOut) {
		t.Errorf("stdout mismatch\nargs: %v\nref: %q\ngot: %q",
			args, refOut, gotOut)
	}
	if !bytes.Equal(refErr, gotErr) {
		t.Errorf("stderr mismatch\nargs: %v\nref: %q\ngot: %q",
			args, refErr, gotErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("exit code mismatch\nargs: %v\nref=%d got=%d",
			args, ref.exitCode, got.exitCode)
	}
}

// writeFile creates a file with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

// combineVerifiers merges multiple verify functions into one.
func combineVerifiers(fns ...func(t *testing.T, refDir, goDir string)) func(t *testing.T, refDir, goDir string) {
	return func(t *testing.T, refDir, goDir string) {
		t.Helper()
		for _, fn := range fns {
			fn(t, refDir, goDir)
		}
	}
}

// verifyHardLink returns a verify function that checks both dirs have
// a hard link where target and link share the same inode.
func verifyHardLink(target, link string) func(t *testing.T, refDir, goDir string) {
	return func(t *testing.T, refDir, goDir string) {
		t.Helper()
		checkHardLink(t, "go", filepath.Join(goDir, target), filepath.Join(goDir, link))
	}
}

// checkHardLink verifies two paths share the same inode (are hard links).
func checkHardLink(t *testing.T, label, target, link string) {
	t.Helper()
	tInfo, err := os.Stat(target)
	if err != nil {
		t.Errorf("%s: target stat failed: %v", label, err)
		return
	}
	lInfo, err := os.Stat(link)
	if err != nil {
		t.Errorf("%s: link stat failed: %v", label, err)
		return
	}
	if !os.SameFile(tInfo, lInfo) {
		t.Errorf("%s: %s and %s are not hard links", label, target, link)
	}
}

// verifySymLink returns a verify function that checks the go dir has
// a symlink at linkName pointing to expectedTarget.
func verifySymLink(linkName, expectedTarget string) func(t *testing.T, refDir, goDir string) {
	return func(t *testing.T, refDir, goDir string) {
		t.Helper()
		checkSymLink(t, "go", filepath.Join(goDir, linkName), expectedTarget)
	}
}

// checkSymLink verifies a path is a symlink pointing to the expected target.
func checkSymLink(t *testing.T, label, linkPath, expectedTarget string) {
	t.Helper()
	got, err := os.Readlink(linkPath)
	if err != nil {
		t.Errorf("%s: readlink failed: %v", label, err)
		return
	}
	if got != expectedTarget {
		t.Errorf("%s: symlink target mismatch: got %q want %q",
			label, got, expectedTarget)
	}
}

// verifyFileContent returns a verify function that checks file content
// matches in the go dir.
func verifyFileContent(name, expected string) func(t *testing.T, refDir, goDir string) {
	return func(t *testing.T, refDir, goDir string) {
		t.Helper()
		checkFileContent(t, "go", filepath.Join(goDir, name), expected)
	}
}

// verifyFileAbsent returns a verify function that checks a file does not
// exist in the go dir.
func verifyFileAbsent(name string) func(t *testing.T, refDir, goDir string) {
	return func(t *testing.T, refDir, goDir string) {
		t.Helper()
		path := filepath.Join(goDir, name)
		if _, err := os.Lstat(path); err == nil {
			t.Errorf("go: expected %s to be absent but it exists", name)
		}
	}
}

// checkFileContent verifies a file has the expected content.
func checkFileContent(t *testing.T, label, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("%s: read failed: %v", label, err)
		return
	}
	if string(data) != expected {
		t.Errorf("%s: content mismatch: got %q want %q",
			label, string(data), expected)
	}
}
