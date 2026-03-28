// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/ln comparing against gln (GNU coreutils).
// Covers prd037-ln R1.1-R1.4 (hard links), R2.1-R2.4 (symbolic links,
// relative), R3.1-R3.6 (force, no-dereference, interactive, verbose,
// backup, suffix), R4.1-R4.3 (differential tests, filesystem state).
package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// cmdResult holds captured output from a binary invocation.
type cmdResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

// stderrNormalizer normalizes error messages between GNU gln and Go ln.
func stderrNormalizer() testutils.NormalizeFunc {
	binName := regexp.MustCompile(`/[^\s:]+/g?ln|gln`)
	tryHelp := regexp.MustCompile(`(?m)^Try '[^']*' for more information\.\n?`)
	return func(b []byte) []byte {
		b = binName.ReplaceAll(b, []byte("ln"))
		b = tryHelp.ReplaceAll(b, nil)
		b = bytes.ToLower(b)
		return b
	}
}

// TestDiffErrors tests error cases that don't mutate filesystem state,
// using RunDiffTests directly.
func TestDiffErrors(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	errNorm := stderrNormalizer()

	// R1.3: create a directory for hard-link-to-directory test.
	dirPath := filepath.Join(t.TempDir(), "testdir")
	if mkErr := os.Mkdir(dirPath, 0o755); mkErr != nil {
		t.Fatalf("create test directory: %v", mkErr)
	}

	tests := []testutils.DiffTest{
		// R1.4: missing operand
		{
			Name:      "no_args",
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// R1.3: hard link to directory fails
		{
			Name:      "hard_link_to_directory",
			Args:      []string{dirPath, filepath.Join(t.TempDir(), "link_to_dir")},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
		// Error: nonexistent target for hard link
		{
			Name:      "hard_link_nonexistent",
			Args:      []string{"no_such_file_xyz", "link_xyz"},
			Normalize: []testutils.NormalizeFunc{errNorm},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// runBinInput runs a binary with args, working directory, and optional stdin.
func runBinInput(
	t *testing.T, binary string, args []string,
	dir string, stdin []byte,
) cmdResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", binary, runErr)
		}
	}
	return cmdResult{
		stdout:   stdout.Bytes(),
		stderr:   stderr.Bytes(),
		exitCode: exitCode,
	}
}

// runBin runs a binary with args in the given working directory.
func runBin(t *testing.T, binary string, args []string, dir string) cmdResult {
	t.Helper()
	return runBinInput(t, binary, args, dir, nil)
}

// compareResults asserts that ref and go binary outputs match.
func compareResults(t *testing.T, name string, ref, got cmdResult) {
	t.Helper()
	norm := stderrNormalizer()
	if !bytes.Equal(ref.stdout, got.stdout) {
		t.Errorf("[%s] stdout mismatch\n  ref: %q\n  go:  %q",
			name, ref.stdout, got.stdout)
	}
	refErr := norm(ref.stderr)
	goErr := norm(got.stderr)
	if !bytes.Equal(refErr, goErr) {
		t.Errorf("[%s] stderr mismatch\n  ref: %q\n  go:  %q",
			name, refErr, goErr)
	}
	if ref.exitCode != got.exitCode {
		t.Errorf("[%s] exit code mismatch: ref=%d go=%d",
			name, ref.exitCode, got.exitCode)
	}
}

// writeFile creates a file with given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", path, err)
	}
}

// mustMkdir creates a directory.
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

// TestHardLinkBasic verifies R1.1: hard link from TARGET to LINK_NAME.
// R4.3: verifies link type (hard).
func TestHardLinkBasic(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "source.txt"), "hello")
	refRes := runBin(t, refBin, []string{"source.txt", "hardlink.txt"}, refDir)
	assertHardLink(t, filepath.Join(refDir, "source.txt"),
		filepath.Join(refDir, "hardlink.txt"), "ref")

	// Go binary
	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "source.txt"), "hello")
	goRes := runBin(t, goBin, []string{"source.txt", "hardlink.txt"}, goDir)
	assertHardLink(t, filepath.Join(goDir, "source.txt"),
		filepath.Join(goDir, "hardlink.txt"), "go")

	compareResults(t, "hard_link_basic", refRes, goRes)
}

// TestHardLinkMultipleIntoDir verifies R1.2: multiple targets into directory.
func TestHardLinkMultipleIntoDir(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "a.txt"), "aaa")
	writeFile(t, filepath.Join(refDir, "b.txt"), "bbb")
	mustMkdir(t, filepath.Join(refDir, "dest"))
	refRes := runBin(t, refBin, []string{"a.txt", "b.txt", "dest"}, refDir)
	assertHardLink(t, filepath.Join(refDir, "a.txt"),
		filepath.Join(refDir, "dest", "a.txt"), "ref-a")
	assertHardLink(t, filepath.Join(refDir, "b.txt"),
		filepath.Join(refDir, "dest", "b.txt"), "ref-b")

	// Go binary
	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "a.txt"), "aaa")
	writeFile(t, filepath.Join(goDir, "b.txt"), "bbb")
	mustMkdir(t, filepath.Join(goDir, "dest"))
	goRes := runBin(t, goBin, []string{"a.txt", "b.txt", "dest"}, goDir)
	assertHardLink(t, filepath.Join(goDir, "a.txt"),
		filepath.Join(goDir, "dest", "a.txt"), "go-a")
	assertHardLink(t, filepath.Join(goDir, "b.txt"),
		filepath.Join(goDir, "dest", "b.txt"), "go-b")

	compareResults(t, "hard_link_multi", refRes, goRes)
}

// TestHardLinkExistingDest verifies R1.4: error when dest exists.
func TestHardLinkExistingDest(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "src")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "dst")
	refRes := runBin(t, refBin, []string{"src.txt", "dst.txt"}, refDir)

	// Go binary
	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "src")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "dst")
	goRes := runBin(t, goBin, []string{"src.txt", "dst.txt"}, goDir)

	compareResults(t, "hard_link_existing", refRes, goRes)
}

// TestSymlinkBasic verifies R2.1: -s creates symbolic link.
// R4.3: verifies link type (symbolic) and target.
func TestSymlinkBasic(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "target.txt"), "content")
	refRes := runBin(t, refBin, []string{"-s", "target.txt", "sym.txt"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "sym.txt"), "target.txt", "ref")

	// Go binary
	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "target.txt"), "content")
	goRes := runBin(t, goBin, []string{"-s", "target.txt", "sym.txt"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "sym.txt"), "target.txt", "go")

	compareResults(t, "symlink_basic", refRes, goRes)
}

// TestSymlinkToDirectory verifies R2.2: symbolic links to directories.
func TestSymlinkToDirectory(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	mustMkdir(t, filepath.Join(refDir, "mydir"))
	refRes := runBin(t, refBin, []string{"-s", "mydir", "dirlink"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "dirlink"), "mydir", "ref")

	// Go binary
	goDir := t.TempDir()
	mustMkdir(t, filepath.Join(goDir, "mydir"))
	goRes := runBin(t, goBin, []string{"-s", "mydir", "dirlink"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "dirlink"), "mydir", "go")

	compareResults(t, "symlink_to_dir", refRes, goRes)
}

// TestSymlinkDangling verifies R2.1: dangling symlink (nonexistent target).
func TestSymlinkDangling(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	// Reference binary
	refDir := t.TempDir()
	refRes := runBin(t, refBin,
		[]string{"-s", "nonexistent", "dangling"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "dangling"), "nonexistent", "ref")

	// Go binary
	goDir := t.TempDir()
	goRes := runBin(t, goBin,
		[]string{"-s", "nonexistent", "dangling"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "dangling"), "nonexistent", "go")

	compareResults(t, "symlink_dangling", refRes, goRes)
}

// TestSymlinkAbsoluteTarget verifies R2.3: absolute target stored as-is.
func TestSymlinkAbsoluteTarget(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	absTarget := "/usr/bin/env"

	refDir := t.TempDir()
	refRes := runBin(t, refBin, []string{"-s", absTarget, "abslink"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "abslink"), absTarget, "ref")

	goDir := t.TempDir()
	goRes := runBin(t, goBin, []string{"-s", absTarget, "abslink"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "abslink"), absTarget, "go")

	compareResults(t, "symlink_absolute_target", refRes, goRes)
}

// TestForceOverwrite verifies R3.1: -f removes existing destination
// before creating a hard link.
func TestForceOverwrite(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "existing")
	refRes := runBin(t, refBin, []string{"-f", "src.txt", "dst.txt"}, refDir)
	assertHardLink(t, filepath.Join(refDir, "src.txt"),
		filepath.Join(refDir, "dst.txt"), "ref")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "existing")
	goRes := runBin(t, goBin, []string{"-f", "src.txt", "dst.txt"}, goDir)
	assertHardLink(t, filepath.Join(goDir, "src.txt"),
		filepath.Join(goDir, "dst.txt"), "go")

	compareResults(t, "force_overwrite", refRes, goRes)
}

// TestForceSymlinkReplace verifies R3.1: -sf replaces existing symlink.
func TestForceSymlinkReplace(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "old.txt"), "old")
	writeFile(t, filepath.Join(refDir, "new.txt"), "new")
	if err := os.Symlink("old.txt", filepath.Join(refDir, "link")); err != nil {
		t.Fatal(err)
	}
	refRes := runBin(t, refBin, []string{"-sf", "new.txt", "link"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "link"), "new.txt", "ref")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "old.txt"), "old")
	writeFile(t, filepath.Join(goDir, "new.txt"), "new")
	if err := os.Symlink("old.txt", filepath.Join(goDir, "link")); err != nil {
		t.Fatal(err)
	}
	goRes := runBin(t, goBin, []string{"-sf", "new.txt", "link"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "link"), "new.txt", "go")

	compareResults(t, "force_symlink_replace", refRes, goRes)
}

// TestNoDereferenceSymlinkDir verifies R3.2: -sfn replaces a symlink
// to a directory rather than creating a link inside the directory.
func TestNoDereferenceSymlinkDir(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	mustMkdir(t, filepath.Join(refDir, "realdir"))
	writeFile(t, filepath.Join(refDir, "target.txt"), "content")
	if err := os.Symlink("realdir", filepath.Join(refDir, "dirlink")); err != nil {
		t.Fatal(err)
	}
	refRes := runBin(t, refBin,
		[]string{"-sfn", "target.txt", "dirlink"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "dirlink"), "target.txt", "ref")

	goDir := t.TempDir()
	mustMkdir(t, filepath.Join(goDir, "realdir"))
	writeFile(t, filepath.Join(goDir, "target.txt"), "content")
	if err := os.Symlink("realdir", filepath.Join(goDir, "dirlink")); err != nil {
		t.Fatal(err)
	}
	goRes := runBin(t, goBin,
		[]string{"-sfn", "target.txt", "dirlink"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "dirlink"), "target.txt", "go")

	compareResults(t, "no_dereference_symlink_dir", refRes, goRes)
}

// TestVerboseHardLink verifies R3.4: -v prints verbose output for hard links.
func TestVerboseHardLink(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "hello")
	refRes := runBin(t, refBin, []string{"-v", "src.txt", "link.txt"}, refDir)

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "hello")
	goRes := runBin(t, goBin, []string{"-v", "src.txt", "link.txt"}, goDir)

	compareResults(t, "verbose_hard_link", refRes, goRes)
}

// TestVerboseSymlink verifies R3.4: -sv prints verbose output for symlinks.
func TestVerboseSymlink(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "target.txt"), "hello")
	refRes := runBin(t, refBin,
		[]string{"-sv", "target.txt", "sym.txt"}, refDir)

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "target.txt"), "hello")
	goRes := runBin(t, goBin,
		[]string{"-sv", "target.txt", "sym.txt"}, goDir)

	compareResults(t, "verbose_symlink", refRes, goRes)
}

// TestRelativeSymlink verifies R2.4: -sr creates a relative symlink
// when link is in a subdirectory.
func TestRelativeSymlink(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "target.txt"), "content")
	mustMkdir(t, filepath.Join(refDir, "sub"))
	refRes := runBin(t, refBin,
		[]string{"-sr", "target.txt", "sub/link.txt"}, refDir)
	refTarget, _ := os.Readlink(filepath.Join(refDir, "sub", "link.txt"))

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "target.txt"), "content")
	mustMkdir(t, filepath.Join(goDir, "sub"))
	goRes := runBin(t, goBin,
		[]string{"-sr", "target.txt", "sub/link.txt"}, goDir)
	goTarget, _ := os.Readlink(filepath.Join(goDir, "sub", "link.txt"))

	compareResults(t, "relative_symlink", refRes, goRes)
	if refTarget != goTarget {
		t.Errorf("relative symlink target mismatch: ref=%q go=%q",
			refTarget, goTarget)
	}
}

// TestRelativeSymlinkSameDir verifies R2.4: -sr in the same directory.
func TestRelativeSymlinkSameDir(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "target.txt"), "content")
	refRes := runBin(t, refBin,
		[]string{"-sr", "target.txt", "link.txt"}, refDir)
	refTarget, _ := os.Readlink(filepath.Join(refDir, "link.txt"))

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "target.txt"), "content")
	goRes := runBin(t, goBin,
		[]string{"-sr", "target.txt", "link.txt"}, goDir)
	goTarget, _ := os.Readlink(filepath.Join(goDir, "link.txt"))

	compareResults(t, "relative_symlink_same_dir", refRes, goRes)
	if refTarget != goTarget {
		t.Errorf("relative symlink target mismatch: ref=%q go=%q",
			refTarget, goTarget)
	}
}

// TestInteractiveAccept verifies R3.3: -i with "y" response replaces dest.
func TestInteractiveAccept(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	yesInput := []byte("y\n")

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "existing")
	refRes := runBinInput(t, refBin,
		[]string{"-i", "src.txt", "dst.txt"}, refDir, yesInput)
	assertHardLink(t, filepath.Join(refDir, "src.txt"),
		filepath.Join(refDir, "dst.txt"), "ref")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "existing")
	goRes := runBinInput(t, goBin,
		[]string{"-i", "src.txt", "dst.txt"}, goDir, yesInput)
	assertHardLink(t, filepath.Join(goDir, "src.txt"),
		filepath.Join(goDir, "dst.txt"), "go")

	compareResults(t, "interactive_accept", refRes, goRes)
}

// TestInteractiveDecline verifies R3.3: -i with "n" response keeps dest.
func TestInteractiveDecline(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	noInput := []byte("n\n")

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "existing")
	refRes := runBinInput(t, refBin,
		[]string{"-i", "src.txt", "dst.txt"}, refDir, noInput)

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "existing")
	goRes := runBinInput(t, goBin,
		[]string{"-i", "src.txt", "dst.txt"}, goDir, noInput)

	compareResults(t, "interactive_decline", refRes, goRes)

	// Verify dest was not replaced in the Go run.
	data, err := os.ReadFile(filepath.Join(goDir, "dst.txt"))
	if err != nil {
		t.Fatalf("read dst.txt: %v", err)
	}
	if string(data) != "existing" {
		t.Errorf("dst.txt was modified despite decline: %q", data)
	}
}

// TestForceInteractiveLastWins verifies R3.3: when -f and -i are both
// given, the last flag on the command line wins.
func TestForceInteractiveLastWins(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	noInput := []byte("n\n")

	// -f then -i: interactive wins, decline should preserve dest.
	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "existing")
	refRes := runBinInput(t, refBin,
		[]string{"-f", "-i", "src.txt", "dst.txt"}, refDir, noInput)

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "existing")
	goRes := runBinInput(t, goBin,
		[]string{"-f", "-i", "src.txt", "dst.txt"}, goDir, noInput)

	compareResults(t, "force_interactive_last_wins", refRes, goRes)
}

// TestBackupSimple verifies R3.5: -b creates a backup with ~ suffix.
// R4.1: compares filesystem state (backup file exists with original content).
func TestBackupSimple(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "original")
	refRes := runBin(t, refBin,
		[]string{"-bf", "src.txt", "dst.txt"}, refDir)
	refBackup := readFileContent(t, filepath.Join(refDir, "dst.txt~"))

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "original")
	goRes := runBin(t, goBin,
		[]string{"-bf", "src.txt", "dst.txt"}, goDir)
	goBackup := readFileContent(t, filepath.Join(goDir, "dst.txt~"))

	compareResults(t, "backup_simple", refRes, goRes)
	assertHardLink(t, filepath.Join(goDir, "src.txt"),
		filepath.Join(goDir, "dst.txt"), "go-link")
	if refBackup != goBackup {
		t.Errorf("backup content mismatch: ref=%q go=%q",
			refBackup, goBackup)
	}
}

// TestBackupNumbered verifies R3.5: --backup=numbered creates numbered
// backups like dst.txt.~1~.
// R4.1: compares filesystem state.
func TestBackupNumbered(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "original")
	refRes := runBin(t, refBin,
		[]string{"-f", "--backup=numbered", "src.txt", "dst.txt"}, refDir)
	assertFileExists(t, filepath.Join(refDir, "dst.txt.~1~"), "ref")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "original")
	goRes := runBin(t, goBin,
		[]string{"-f", "--backup=numbered", "src.txt", "dst.txt"}, goDir)
	assertFileExists(t, filepath.Join(goDir, "dst.txt.~1~"), "go")

	compareResults(t, "backup_numbered", refRes, goRes)
}

// TestBackupNumberedIncrement verifies R3.5: numbered backups increment.
// R4.1: compares that ~1~ and ~2~ both exist.
func TestBackupNumberedIncrement(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "v1")
	runBin(t, refBin,
		[]string{"-f", "--backup=numbered", "src.txt", "dst.txt"}, refDir)
	writeFile(t, filepath.Join(refDir, "dst.txt"), "v2")
	refRes := runBin(t, refBin,
		[]string{"-f", "--backup=numbered", "src.txt", "dst.txt"}, refDir)
	assertFileExists(t, filepath.Join(refDir, "dst.txt.~1~"), "ref-1")
	assertFileExists(t, filepath.Join(refDir, "dst.txt.~2~"), "ref-2")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "v1")
	runBin(t, goBin,
		[]string{"-f", "--backup=numbered", "src.txt", "dst.txt"}, goDir)
	writeFile(t, filepath.Join(goDir, "dst.txt"), "v2")
	goRes := runBin(t, goBin,
		[]string{"-f", "--backup=numbered", "src.txt", "dst.txt"}, goDir)
	assertFileExists(t, filepath.Join(goDir, "dst.txt.~1~"), "go-1")
	assertFileExists(t, filepath.Join(goDir, "dst.txt.~2~"), "go-2")

	compareResults(t, "backup_numbered_increment", refRes, goRes)
}

// TestBackupExistingFallsToSimple verifies R3.5: --backup=existing
// uses simple backup when no numbered backups exist.
func TestBackupExistingFallsToSimple(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "original")
	refRes := runBin(t, refBin,
		[]string{"-f", "--backup=existing", "src.txt", "dst.txt"}, refDir)
	assertFileExists(t, filepath.Join(refDir, "dst.txt~"), "ref")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "original")
	goRes := runBin(t, goBin,
		[]string{"-f", "--backup=existing", "src.txt", "dst.txt"}, goDir)
	assertFileExists(t, filepath.Join(goDir, "dst.txt~"), "go")

	compareResults(t, "backup_existing_simple", refRes, goRes)
}

// TestBackupExistingUsesNumbered verifies R3.5: --backup=existing uses
// numbered backup when numbered backups already exist.
func TestBackupExistingUsesNumbered(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "original")
	// Create a numbered backup so existing mode detects it.
	writeFile(t, filepath.Join(refDir, "dst.txt.~1~"), "backup1")
	refRes := runBin(t, refBin,
		[]string{"-f", "--backup=existing", "src.txt", "dst.txt"}, refDir)
	assertFileExists(t, filepath.Join(refDir, "dst.txt.~2~"), "ref")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "original")
	writeFile(t, filepath.Join(goDir, "dst.txt.~1~"), "backup1")
	goRes := runBin(t, goBin,
		[]string{"-f", "--backup=existing", "src.txt", "dst.txt"}, goDir)
	assertFileExists(t, filepath.Join(goDir, "dst.txt.~2~"), "go")

	compareResults(t, "backup_existing_numbered", refRes, goRes)
}

// TestBackupNone verifies R3.5: --backup=none skips backup creation.
func TestBackupNone(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "original")
	refRes := runBin(t, refBin,
		[]string{"-f", "--backup=none", "src.txt", "dst.txt"}, refDir)
	assertFileNotExists(t, filepath.Join(refDir, "dst.txt~"), "ref")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "original")
	goRes := runBin(t, goBin,
		[]string{"-f", "--backup=none", "src.txt", "dst.txt"}, goDir)
	assertFileNotExists(t, filepath.Join(goDir, "dst.txt~"), "go")

	compareResults(t, "backup_none", refRes, goRes)
}

// TestBackupCustomSuffix verifies R3.6: -S changes the backup suffix.
// R4.1: compares filesystem state.
func TestBackupCustomSuffix(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "original")
	refRes := runBin(t, refBin,
		[]string{"-bf", "-S", ".bak", "src.txt", "dst.txt"}, refDir)
	assertFileExists(t, filepath.Join(refDir, "dst.txt.bak"), "ref")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "original")
	goRes := runBin(t, goBin,
		[]string{"-bf", "-S", ".bak", "src.txt", "dst.txt"}, goDir)
	assertFileExists(t, filepath.Join(goDir, "dst.txt.bak"), "go")

	compareResults(t, "backup_custom_suffix", refRes, goRes)
}

// TestBackupSuffixLongFlag verifies R3.6: --suffix=SUFFIX variant.
func TestBackupSuffixLongFlag(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "src.txt"), "source")
	writeFile(t, filepath.Join(refDir, "dst.txt"), "original")
	refRes := runBin(t, refBin,
		[]string{"-bf", "--suffix=.orig", "src.txt", "dst.txt"}, refDir)
	assertFileExists(t, filepath.Join(refDir, "dst.txt.orig"), "ref")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "src.txt"), "source")
	writeFile(t, filepath.Join(goDir, "dst.txt"), "original")
	goRes := runBin(t, goBin,
		[]string{"-bf", "--suffix=.orig", "src.txt", "dst.txt"}, goDir)
	assertFileExists(t, filepath.Join(goDir, "dst.txt.orig"), "go")

	compareResults(t, "backup_suffix_long", refRes, goRes)
}

// TestBackupSymlink verifies R3.5: -b with symbolic link creation.
// R4.3: verifies the resulting link is symbolic.
func TestBackupSymlink(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gln")
	if err != nil {
		t.Skipf("reference binary gln not in PATH: %v", err)
	}

	refDir := t.TempDir()
	writeFile(t, filepath.Join(refDir, "target.txt"), "content")
	writeFile(t, filepath.Join(refDir, "link.txt"), "old")
	refRes := runBin(t, refBin,
		[]string{"-sbf", "target.txt", "link.txt"}, refDir)
	assertSymlink(t, filepath.Join(refDir, "link.txt"), "target.txt", "ref")
	assertFileExists(t, filepath.Join(refDir, "link.txt~"), "ref-backup")

	goDir := t.TempDir()
	writeFile(t, filepath.Join(goDir, "target.txt"), "content")
	writeFile(t, filepath.Join(goDir, "link.txt"), "old")
	goRes := runBin(t, goBin,
		[]string{"-sbf", "target.txt", "link.txt"}, goDir)
	assertSymlink(t, filepath.Join(goDir, "link.txt"), "target.txt", "go")
	assertFileExists(t, filepath.Join(goDir, "link.txt~"), "go-backup")

	compareResults(t, "backup_symlink", refRes, goRes)
}

// assertHardLink verifies two paths share the same inode.
// R4.3: verifies link type is hard (same file).
func assertHardLink(t *testing.T, src, dst, label string) {
	t.Helper()
	srcInfo, err := os.Stat(src)
	if err != nil {
		t.Errorf("[%s] stat source: %v", label, err)
		return
	}
	dstInfo, err := os.Stat(dst)
	if err != nil {
		t.Errorf("[%s] stat dest: %v", label, err)
		return
	}
	if !os.SameFile(srcInfo, dstInfo) {
		t.Errorf("[%s] %s and %s are not the same file", label, src, dst)
	}
}

// assertSymlink verifies a symlink points to the expected target.
// R4.3: verifies link type is symbolic and target matches.
func assertSymlink(t *testing.T, link, expectedTarget, label string) {
	t.Helper()
	target, err := os.Readlink(link)
	if err != nil {
		t.Errorf("[%s] readlink %s: %v", label, link, err)
		return
	}
	if target != expectedTarget {
		t.Errorf("[%s] symlink target = %q, want %q",
			label, target, expectedTarget)
	}
}

// assertFileExists verifies a file exists at the given path.
// R4.1: filesystem state comparison.
func assertFileExists(t *testing.T, path, label string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Errorf("[%s] expected file %s to exist: %v", label, path, err)
	}
}

// assertFileNotExists verifies no file exists at the given path.
func assertFileNotExists(t *testing.T, path, label string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Errorf("[%s] expected file %s to not exist", label, path)
	}
}

// readFileContent reads file content as a string.
func readFileContent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readFileContent %s: %v", path, err)
	}
	return string(data)
}
