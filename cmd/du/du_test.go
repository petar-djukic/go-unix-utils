// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/du comparing Go binary against GNU gdu reference.
//
// Implements: prd009-du R1, R2, R3, R4, R5
// Traces: test-rel01.3, rel01.3-uc001-du
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// goBinaryPath is the path to the compiled Go du binary.
var goBinaryPath string

// refBinaryPath is the path to the GNU reference binary (gdu).
var refBinaryPath string

func TestMain(m *testing.M) {
	// Locate GNU reference binary.
	ref, err := exec.LookPath("gdu")
	if err != nil {
		fmt.Println("gdu not found; skipping du differential tests")
		os.Exit(0)
	}
	refBinaryPath = ref

	// Build the Go du binary to a temp directory.
	tmpDir, err := os.MkdirTemp("", "du-test-*")
	if err != nil {
		panic("creating temp dir: " + err.Error())
	}

	goBinaryPath = filepath.Join(tmpDir, "du")
	cmd := exec.Command("go", "build", "-o", goBinaryPath, ".")
	cmd.Dir = filepath.Join(".")
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(tmpDir)
		panic(fmt.Sprintf("building du binary: %v\n%s", err, out))
	}

	code := m.Run()
	os.RemoveAll(tmpDir) // best-effort cleanup
	os.Exit(code)
}

// writeFile creates a file with the given content in dir and returns the filename.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
	return name
}

// makeFixtureTree creates a nested directory tree for du tests:
//
//	root/
//	  subdir1/
//	    file1.txt (1024 bytes)
//	  subdir2/
//	    file2.txt (2048 bytes)
//	    nested/
//	      file3.txt (512 bytes)
func makeFixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	subdir1 := filepath.Join(root, "subdir1")
	subdir2 := filepath.Join(root, "subdir2")
	nested := filepath.Join(subdir2, "nested")

	for _, d := range []string{subdir1, subdir2, nested} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatalf("creating directory %s: %v", d, err)
		}
	}

	if err := os.WriteFile(filepath.Join(subdir1, "file1.txt"), make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("writing file1.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subdir2, "file2.txt"), make([]byte, 2048), 0o644); err != nil {
		t.Fatalf("writing file2.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "file3.txt"), make([]byte, 512), 0o644); err != nil {
		t.Fatalf("writing file3.txt: %v", err)
	}

	return root
}

// --- Default traversal (prd009-du R1.1, R1.3, R1.4, R1.5) ---

func TestDefault_Recursive(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "default-recursive",
			Args:    []string{"-k", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestDefault_MultipleArgs(t *testing.T) {
	root := makeFixtureTree(t)
	subdir1 := filepath.Join(root, "subdir1")
	subdir2 := filepath.Join(root, "subdir2")

	tests := []testutils.DiffTest{
		{
			Name:    "multiple-args-in-order",
			Args:    []string{"-k", subdir1, subdir2},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

func TestDefault_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	emptyDir := filepath.Join(dir, "empty")
	if err := os.Mkdir(emptyDir, 0o755); err != nil {
		t.Fatalf("creating empty dir: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:    "empty-directory",
			Args:    []string{"-k", emptyDir},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Non-directory argument (prd009-du R1.5) ---

func TestDefault_NonDirectoryArg(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "single.txt"), make([]byte, 4096), 0o644); err != nil {
		t.Fatalf("writing single.txt: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:    "non-directory-argument",
			Args:    []string{"-k", filepath.Join(dir, "single.txt")},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Human-readable -h (prd009-du R2.1) ---

func TestFlag_HumanReadable(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "human-readable",
			Args:    []string{"-h", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Summarize -s (prd009-du R2.2) ---

func TestFlag_Summarize(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "summarize",
			Args:    []string{"-sk", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- All files -a (prd009-du R2.3) ---

func TestFlag_AllFiles(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "all-files",
			Args:    []string{"-ak", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Depth limiting -d (prd009-du R2.4) ---

func TestFlag_MaxDepth(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "max-depth-0",
			Args:    []string{"-k", "-d", "0", root},
			WorkDir: root,
		},
		{
			Name:    "max-depth-1",
			Args:    []string{"-k", "-d", "1", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Kilobyte units -k (prd009-du R2.5) ---

func TestFlag_Kilobyte(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "kilobyte-flag",
			Args:    []string{"-k", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Megabyte units -m (prd009-du R2.6) ---

func TestFlag_Megabyte(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "megabyte-flag",
			Args:    []string{"-m", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Grand total -c (prd009-du R2.7) ---

func TestFlag_GrandTotal(t *testing.T) {
	root := makeFixtureTree(t)
	subdir1 := filepath.Join(root, "subdir1")
	subdir2 := filepath.Join(root, "subdir2")

	tests := []testutils.DiffTest{
		{
			Name:    "grand-total",
			Args:    []string{"-ck", subdir1, subdir2},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Apparent size --apparent-size (prd009-du R2.8) ---

func TestFlag_ApparentSize(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "apparent-size",
			Args:    []string{"--apparent-size", "-k", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Byte mode -b (prd009-du R2.8) ---

func TestFlag_ByteMode(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "byte-mode",
			Args:    []string{"-b", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Hard-link deduplication (prd009-du R3.1, R3.2, R3.3) ---

func TestHardLink_Dedup(t *testing.T) {
	root := t.TempDir()
	dirA := filepath.Join(root, "dir_a")
	dirB := filepath.Join(root, "dir_b")

	if err := os.Mkdir(dirA, 0o755); err != nil {
		t.Fatalf("creating dir_a: %v", err)
	}
	if err := os.Mkdir(dirB, 0o755); err != nil {
		t.Fatalf("creating dir_b: %v", err)
	}

	// Create original file and hard link it into a second directory.
	origPath := filepath.Join(dirA, "shared.txt")
	if err := os.WriteFile(origPath, make([]byte, 4096), 0o644); err != nil {
		t.Fatalf("writing shared.txt: %v", err)
	}
	linkPath := filepath.Join(dirB, "shared.txt")
	if err := os.Link(origPath, linkPath); err != nil {
		t.Skipf("os.Link not supported: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:    "hardlink-dedup",
			Args:    []string{"-ak", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Error handling: missing path (prd009-du R4.2) ---

func TestError_MissingPath(t *testing.T) {
	dir := t.TempDir()

	tests := []testutils.DiffTest{
		{
			Name:    "missing-path",
			Args:    []string{"-k", filepath.Join(dir, "nonexistent_dir")},
			WorkDir: dir,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Error handling: mixed valid and invalid paths (prd009-du R4.2) ---

func TestError_MixedValidInvalid(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "mixed-valid-invalid",
			Args:    []string{"-k", filepath.Join(root, "nonexistent"), filepath.Join(root, "subdir1")},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Combined flags (prd009-du R2.1, R2.7) ---

func TestFlag_Combined_HC(t *testing.T) {
	root := makeFixtureTree(t)
	subdir1 := filepath.Join(root, "subdir1")
	subdir2 := filepath.Join(root, "subdir2")

	tests := []testutils.DiffTest{
		{
			Name:    "combined-h-c",
			Args:    []string{"-h", "-c", subdir1, subdir2},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}

// --- Combined -a with -d (prd009-du R2.3, R2.4) ---

func TestFlag_Combined_AD(t *testing.T) {
	root := makeFixtureTree(t)

	tests := []testutils.DiffTest{
		{
			Name:    "combined-a-d1",
			Args:    []string{"-ak", "-d", "1", root},
			WorkDir: root,
		},
	}
	testutils.RunDiffTests(t, goBinaryPath, refBinaryPath, tests)
}
