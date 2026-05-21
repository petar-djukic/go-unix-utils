package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
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

// R2.1: -i LO-HI generates and shuffles integers from LO to HI inclusive
func TestRangeMode(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-i", "1-10")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -i 1-10 failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(got))
	}

	seen := make(map[int]bool)
	for _, line := range got {
		v, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("non-integer in output: %q", line)
		}
		if v < 1 || v > 10 {
			t.Errorf("value %d out of range [1,10]", v)
		}
		if seen[v] {
			t.Errorf("duplicate value %d without -r", v)
		}
		seen[v] = true
	}
}

// R2.1: --input-range=LO-HI long form
func TestRangeModeLong(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--input-range=3-7")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf --input-range=3-7 failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(got))
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	want := []string{"3", "4", "5", "6", "7"}
	sort.Strings(want)
	if !slices.Equal(sorted, want) {
		t.Errorf("output is not a permutation of 3..7: got %v", got)
	}
}

// R2.1: -i must not be combined with file arguments
func TestRangeWithFileError(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-i", "1-5", "somefile")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when combining -i with file args")
	}
	if !strings.Contains(string(out), "extra operand") {
		t.Errorf("expected 'extra operand' error, got: %s", out)
	}
}

// R2.2: -n COUNT limits output to at most COUNT lines
func TestHeadCount(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	input := "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n"
	cmd := exec.Command(bin, "-n", "3")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -n 3 failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(got))
	}

	valid := map[string]bool{
		"a": true, "b": true, "c": true, "d": true, "e": true,
		"f": true, "g": true, "h": true, "i": true, "j": true,
	}
	for _, line := range got {
		if !valid[line] {
			t.Errorf("unexpected output line: %q", line)
		}
	}
}

// R2.2: -n COUNT with range mode
func TestHeadCountRange(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-i", "1-100", "-n", "5")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -i 1-100 -n 5 failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(got))
	}

	seen := make(map[int]bool)
	for _, line := range got {
		v, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("non-integer: %q", line)
		}
		if v < 1 || v > 100 {
			t.Errorf("value %d out of range [1,100]", v)
		}
		if seen[v] {
			t.Errorf("duplicate value %d without -r", v)
		}
		seen[v] = true
	}
}

// R2.2: -n larger than input produces all lines
func TestHeadCountLargerThanInput(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	input := "x\ny\n"
	cmd := exec.Command(bin, "-n", "100")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -n 100 failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 2 {
		t.Fatalf("expected 2 lines (all input), got %d", len(got))
	}
}

// R2.3: -r with -n produces exactly n lines with possible duplicates
func TestRepeatWithHead(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-i", "1-3", "-r", "-n", "20")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -i 1-3 -r -n 20 failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 20 {
		t.Fatalf("expected 20 lines, got %d", len(got))
	}

	for _, line := range got {
		v, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("non-integer: %q", line)
		}
		if v < 1 || v > 3 {
			t.Errorf("value %d out of range [1,3]", v)
		}
	}
}

// R2.3: -r with stdin input
func TestRepeatStdin(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-r", "-n", "15")
	cmd.Stdin = strings.NewReader("alpha\nbeta\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -r -n 15 failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 15 {
		t.Fatalf("expected 15 lines, got %d", len(got))
	}

	valid := map[string]bool{"alpha": true, "beta": true}
	for _, line := range got {
		if !valid[line] {
			t.Errorf("unexpected output: %q", line)
		}
	}
}

// R2.4: -o FILE writes output to file instead of stdout
func TestOutputFile(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	cmd := exec.Command(bin, "-i", "1-5", "-o", outFile)
	if err := cmd.Run(); err != nil {
		t.Fatalf("shuf -o failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(got) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(got))
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	want := []string{"1", "2", "3", "4", "5"}
	sort.Strings(want)
	if !slices.Equal(sorted, want) {
		t.Errorf("output is not a permutation of 1..5: got %v", got)
	}
}

// R2.4: --output=FILE long form
func TestOutputFileLong(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")

	input := "foo\nbar\nbaz\n"
	cmd := exec.Command(bin, "--output="+outFile)
	cmd.Stdin = strings.NewReader(input)
	if err := cmd.Run(); err != nil {
		t.Fatalf("shuf --output failed: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(got) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(got))
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	want := []string{"bar", "baz", "foo"}
	if !slices.Equal(sorted, want) {
		t.Errorf("output is not a permutation of input: got %v", got)
	}
}

// R2.1: invalid range format
func TestInvalidRange(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-i", "abc")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for invalid range")
	}
	if !strings.Contains(string(out), "invalid input range") {
		t.Errorf("expected 'invalid input range' error, got: %s", out)
	}
}

// R2.1: reversed range (LO > HI)
func TestReversedRange(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-i", "10-5")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for reversed range")
	}
	if !strings.Contains(string(out), "invalid input range") {
		t.Errorf("expected 'invalid input range' error, got: %s", out)
	}
}

// R2.2: -n 0 produces no output
func TestHeadCountZero(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-i", "1-10", "-n", "0")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -n 0 failed: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty output, got: %q", out)
	}
}

// R2.1: differential test for range mode (structural)
func TestDiffRange(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshuf")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:      "range_1_5",
			Args:      []string{"-i", "1-5"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "range_with_head",
			Args:      []string{"-i", "1-100", "-n", "3"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{lineCount},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func lineCount(b []byte) []byte {
	s := strings.TrimSuffix(string(b), "\n")
	if s == "" {
		return []byte("0\n")
	}
	n := len(strings.Split(s, "\n"))
	return []byte(strconv.Itoa(n) + "\n")
}

// R3.1: --random-source=FILE reads random bytes from file
func TestRandomSource(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	dir := t.TempDir()
	rsFile := filepath.Join(dir, "rand.bin")
	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = byte(i * 7)
	}
	if err := os.WriteFile(rsFile, seed, 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bin, "--random-source="+rsFile, "-i", "1-10")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf --random-source failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(got))
	}

	seen := make(map[int]bool)
	for _, line := range got {
		v, err := strconv.Atoi(line)
		if err != nil {
			t.Fatalf("non-integer: %q", line)
		}
		if v < 1 || v > 10 {
			t.Errorf("value %d out of range", v)
		}
		if seen[v] {
			t.Errorf("duplicate: %d", v)
		}
		seen[v] = true
	}

	cmd2 := exec.Command(bin, "--random-source="+rsFile, "-i", "1-10")
	out2, err := cmd2.Output()
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	if string(out) != string(out2) {
		t.Errorf("same random source should produce same output")
	}
}

// R3.1: --random-source with missing file
func TestRandomSourceMissing(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--random-source=/nonexistent", "-i", "1-5")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for missing random source file")
	}
}

// R3.2: -z uses NUL as delimiter for both input and output
func TestZeroTerminated(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	input := "alpha\x00beta\x00gamma\x00"
	cmd := exec.Command(bin, "-z")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -z failed: %v", err)
	}

	raw := string(out)
	raw = strings.TrimSuffix(raw, "\x00")
	got := strings.Split(raw, "\x00")
	if len(got) != 3 {
		t.Fatalf("expected 3 NUL-separated entries, got %d: %q", len(got), out)
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	want := []string{"alpha", "beta", "gamma"}
	if !slices.Equal(sorted, want) {
		t.Errorf("expected permutation of %v, got %v", want, got)
	}
}

// R3.2: --zero-terminated long form
func TestZeroTerminatedLong(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	input := "x\x00y\x00z\x00"
	cmd := exec.Command(bin, "--zero-terminated")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf --zero-terminated failed: %v", err)
	}

	raw := strings.TrimSuffix(string(out), "\x00")
	got := strings.Split(raw, "\x00")
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	want := []string{"x", "y", "z"}
	if !slices.Equal(sorted, want) {
		t.Errorf("expected permutation of %v, got %v", want, got)
	}
}

// R3.2: -z with range mode
func TestZeroTerminatedRange(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-z", "-i", "1-3")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -z -i 1-3 failed: %v", err)
	}

	raw := strings.TrimSuffix(string(out), "\x00")
	got := strings.Split(raw, "\x00")
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	want := []string{"1", "2", "3"}
	if !slices.Equal(sorted, want) {
		t.Errorf("expected permutation of %v, got %v", want, got)
	}
}

// R3.3: -e treats each argument as an input line
func TestEchoMode(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-e", "alpha", "beta", "gamma")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -e failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(got))
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	want := []string{"alpha", "beta", "gamma"}
	if !slices.Equal(sorted, want) {
		t.Errorf("expected permutation of %v, got %v", want, got)
	}
}

// R3.3: --echo long form
func TestEchoModeLong(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "--echo", "one", "two")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf --echo failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(got))
	}

	sorted := slices.Clone(got)
	sort.Strings(sorted)
	want := []string{"one", "two"}
	if !slices.Equal(sorted, want) {
		t.Errorf("expected permutation of %v, got %v", want, got)
	}
}

// R3.3: -e with -n limits output
func TestEchoModeWithHead(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-e", "-n", "2", "a", "b", "c", "d")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -e -n 2 failed: %v", err)
	}

	got := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if len(got) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(got))
	}

	valid := map[string]bool{"a": true, "b": true, "c": true, "d": true}
	for _, line := range got {
		if !valid[line] {
			t.Errorf("unexpected output: %q", line)
		}
	}
}

// R3.3: -e with -i is an error
func TestEchoWithRangeError(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-e", "-i", "1-5", "foo")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error combining -e and -i")
	}
	if !strings.Contains(string(out), "cannot combine") {
		t.Errorf("expected 'cannot combine' error, got: %s", out)
	}
}

// R3.4: empty input produces no output and exits 0
func TestEmptyInput(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin)
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf with empty input failed: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected no output, got: %q", out)
	}
}

// R3.4: -e with no args produces no output
func TestEchoModeEmpty(t *testing.T) {
	bin := testutils.BuildBinary(t, ".")

	cmd := exec.Command(bin, "-e")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("shuf -e with no args failed: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected no output, got: %q", out)
	}
}

// R3.3: differential test for echo mode
func TestDiffEcho(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshuf")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tests := []testutils.DiffTest{
		{
			Name:      "echo_three_words",
			Args:      []string{"-e", "alpha", "beta", "gamma"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "echo_empty",
			Args:      []string{"-e"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortLines},
		},
		{
			Name:      "echo_with_head",
			Args:      []string{"-e", "-n", "2", "a", "b", "c"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{lineCount},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// R3.2: differential test for zero-terminated mode
func TestDiffZeroTerminated(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gshuf")
	if err != nil {
		t.Skip("reference binary not found")
	}

	sortNulLines := func(b []byte) []byte {
		s := strings.TrimSuffix(string(b), "\x00")
		if s == "" {
			return b
		}
		parts := strings.Split(s, "\x00")
		sort.Strings(parts)
		return []byte(strings.Join(parts, "\x00") + "\x00")
	}

	tests := []testutils.DiffTest{
		{
			Name:      "zero_terminated_stdin",
			Args:      []string{"-z"},
			Stdin:     []byte("a\x00b\x00c\x00"),
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortNulLines},
		},
		{
			Name:      "zero_terminated_range",
			Args:      []string{"-z", "-i", "1-3"},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{sortNulLines},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}
