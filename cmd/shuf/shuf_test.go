package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func sortLines(b []byte) []byte {
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return b
	}
	lines := strings.Split(s, "\n")
	sort.Strings(lines)
	return []byte(strings.Join(lines, "\n") + "\n")
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshuf")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:      "empty_input",
			Stdin:     []byte(""),
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "single_line",
			Stdin:     []byte("hello\n"),
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "multiple_lines",
			Stdin:     []byte("alpha\nbeta\ngamma\n"),
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "no_trailing_newline",
			Stdin:     []byte("x\ny\nz"),
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestDiffFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshuf")
	if err != nil {
		t.Skip("reference binary not found")
	}

	dir := t.TempDir()
	f := filepath.Join(dir, "input.txt")
	if err := os.WriteFile(f, []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "file_argument",
			Args:      []string{f},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// R1.1: multiple file arguments concatenate lines then shuffle
func TestMultipleFiles(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	f1 := filepath.Join(dir, "a.txt")
	f2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(f1, []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("c\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, f1, f2)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	want := []string{"a", "b", "c", "d"}

	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d", len(want), len(got))
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	if !slices.Equal(sorted, want) {
		t.Errorf("output is not a permutation of combined input: got %v", got)
	}
}

func TestDiffStdinDash(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshuf")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:      "stdin_via_dash",
			Args:      []string{"-"},
			Stdin:     []byte("p\nq\nr\n"),
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// R1.3: structural verification that output is a permutation
func TestPermutation(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	input := "line1\nline2\nline3\nline4\nline5\n"
	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	want := []string{"line1", "line2", "line3", "line4", "line5"}

	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d", len(want), len(got))
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	if !slices.Equal(sorted, want) {
		t.Errorf("output is not a permutation of input: got %v", got)
	}
}

// R1.4: last line without trailing newline is included
func TestNoTrailingNewline(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	input := "a\nb\nc"
	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	want := []string{"a", "b", "c"}

	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d: %v", len(want), len(got), got)
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	if !slices.Equal(sorted, want) {
		t.Errorf("output is not a permutation of input: got %v", got)
	}
}
