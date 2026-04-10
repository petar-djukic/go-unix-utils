// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// normalizeProgramName strips path prefixes from "sort:" in error messages
// so that "/opt/homebrew/bin/sort:" and "gsort:" both become "sort:".
var normalizeProgramName testutils.NormalizeFunc = func(b []byte) []byte {
	idx := bytes.Index(b, []byte("sort:"))
	if idx > 0 {
		return b[idx:]
	}
	return b
}

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skip("reference binary gsort not in PATH")
	}

	tests := []testutils.DiffTest{
		// R1.5: -u outputs only the first of an equal run
		{
			Name:  "unique removes duplicates",
			Args:  []string{"-u"},
			Stdin: []byte("banana\napple\napple\ncherry\ncherry\n"),
		},
		{
			Name:  "unique with reverse",
			Args:  []string{"-u", "-r"},
			Stdin: []byte("banana\napple\napple\ncherry\ncherry\n"),
		},
		{
			Name:  "unique all same lines",
			Args:  []string{"-u"},
			Stdin: []byte("aaa\naaa\naaa\n"),
		},
		{
			Name:  "unique already unique input",
			Args:  []string{"-u"},
			Stdin: []byte("apple\nbanana\ncherry\n"),
		},
		{
			Name:  "unique empty input",
			Args:  []string{"-u"},
			Stdin: []byte(""),
		},
		// R1.6: -o FILE writes output to FILE
		// (tested via file-based approach below)

		// R1.7: -s preserves input order of equal lines
		{
			Name:  "stable sort preserves order of equal lines",
			Args:  []string{"-s"},
			Stdin: []byte("banana\napple\ncherry\n"),
		},
		{
			Name:  "stable sort with reverse",
			Args:  []string{"-s", "-r"},
			Stdin: []byte("banana\napple\ncherry\n"),
		},
		{
			Name:  "stable sort with unique",
			Args:  []string{"-s", "-u"},
			Stdin: []byte("b\na\na\nc\nc\n"),
		},
		// R2.1: -n numeric sort
		{
			Name:  "numeric sort basic",
			Args:  []string{"-n"},
			Stdin: []byte("10\n2\n1\n"),
		},
		{
			Name:  "numeric sort with negatives",
			Args:  []string{"-n"},
			Stdin: []byte("5\n-3\n0\n10\n-1\n"),
		},
		{
			Name:  "numeric sort with leading spaces",
			Args:  []string{"-n"},
			Stdin: []byte("  10\n2\n  1\n"),
		},
		{
			Name:  "numeric sort non-numeric lines",
			Args:  []string{"-n"},
			Stdin: []byte("abc\n3\n1\nxyz\n"),
		},
		{
			Name:  "numeric sort with decimals",
			Args:  []string{"-n"},
			Stdin: []byte("1.5\n1.1\n2.0\n0.5\n"),
		},
		{
			Name:  "numeric sort reverse",
			Args:  []string{"-n", "-r"},
			Stdin: []byte("10\n2\n1\n"),
		},
		{
			Name:  "numeric sort unique",
			Args:  []string{"-n", "-u"},
			Stdin: []byte("2\n1\n2\n3\n1\n"),
		},
		// R2.2: -h human-numeric sort
		{
			Name:  "human numeric sort SI suffixes",
			Args:  []string{"-h"},
			Stdin: []byte("1K\n1M\n1G\n500\n"),
		},
		{
			Name:  "human numeric sort mixed values",
			Args:  []string{"-h"},
			Stdin: []byte("10K\n5M\n1G\n100\n2K\n"),
		},
		{
			Name:  "human numeric sort same suffix",
			Args:  []string{"-h"},
			Stdin: []byte("5K\n1K\n10K\n2K\n"),
		},
		// R2.3: -M month sort
		{
			Name:  "month sort basic",
			Args:  []string{"-M"},
			Stdin: []byte("MAR\nJAN\nFEB\nDEC\n"),
		},
		{
			Name:  "month sort with unknown",
			Args:  []string{"-M"},
			Stdin: []byte("FOO\nJAN\nBAR\nFEB\n"),
		},
		{
			Name:  "month sort case insensitive",
			Args:  []string{"-M"},
			Stdin: []byte("mar\njan\nfeb\ndec\n"),
		},
		// R2.4: -V version sort
		{
			Name:  "version sort basic",
			Args:  []string{"-V"},
			Stdin: []byte("file10\nfile2\nfile1\n"),
		},
		{
			Name:  "version sort with dots",
			Args:  []string{"-V"},
			Stdin: []byte("1.10\n1.2\n1.1\n"),
		},
		{
			Name:  "version sort mixed",
			Args:  []string{"-V"},
			Stdin: []byte("v2.0\nv1.10\nv1.2\nv1.1\n"),
		},
		// R3.1: -t field separator
		{
			Name:  "field separator colon sort by field 2",
			Args:  []string{"-t:", "-k2,2"},
			Stdin: []byte("b:2\na:3\nc:1\n"),
		},
		// R3.2: -k key field specs
		{
			Name:  "key field numeric sort",
			Args:  []string{"-t:", "-k2,2n"},
			Stdin: []byte("alice:30\nbob:25\ncharlie:35\n"),
		},
		{
			Name:  "key with character position",
			Args:  []string{"-k1.2,1.2"},
			Stdin: []byte("ba\nac\ncb\n"),
		},
		{
			Name:  "key sort by second blank-delimited field",
			Args:  []string{"-k2,2"},
			Stdin: []byte("foo cherry\nbar apple\nbaz banana\n"),
		},
		// R3.3: multiple -k options
		{
			Name:  "multiple keys primary and tiebreak",
			Args:  []string{"-t:", "-k1,1", "-k2,2n"},
			Stdin: []byte("a:3\nb:1\na:1\nb:2\n"),
		},
		{
			Name:  "multiple keys secondary tiebreak",
			Args:  []string{"-t:", "-k2,2", "-k1,1"},
			Stdin: []byte("b:1\na:2\nc:1\na:1\n"),
		},
		// R3.4: -b ignore leading blanks
		{
			Name:  "ignore leading blanks global",
			Args:  []string{"-b"},
			Stdin: []byte("  cherry\napple\n  banana\n"),
		},
		{
			Name:  "ignore leading blanks with key",
			Args:  []string{"-b", "-k1,1"},
			Stdin: []byte("  cherry\napple\n  banana\n"),
		},
		{
			Name:  "per-key b option in keydef",
			Args:  []string{"-k1,1b"},
			Stdin: []byte("  cherry\napple\n  banana\n"),
		},
		// R4.1: exit 0 when input is sorted successfully
		{
			Name:  "default sort exits 0",
			Args:  []string{},
			Stdin: []byte("banana\napple\ncherry\n"),
		},
		{
			Name:  "empty input exits 0",
			Args:  []string{},
			Stdin: []byte(""),
		},
		// R4.2: -c check sorted order
		{
			Name:     "check sorted input exits 0",
			Args:     []string{"-c"},
			Stdin:    []byte("a\nb\nc\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unsorted input exits 1",
			Args:      []string{"-c"},
			Stdin:     []byte("b\na\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:     "check quiet unsorted exits 1 no stderr",
			Args:     []string{"-C"},
			Stdin:    []byte("b\na\nc\n"),
			ExitCode: 1,
		},
		{
			Name:     "check sorted reverse",
			Args:     []string{"-c", "-r"},
			Stdin:    []byte("c\nb\na\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unsorted reverse exits 1",
			Args:      []string{"-c", "-r"},
			Stdin:     []byte("a\nb\nc\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:     "check sorted numeric",
			Args:     []string{"-c", "-n"},
			Stdin:    []byte("1\n2\n10\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unsorted numeric exits 1",
			Args:      []string{"-c", "-n"},
			Stdin:     []byte("10\n2\n1\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:     "check unique sorted no duplicates",
			Args:     []string{"-c", "-u"},
			Stdin:    []byte("a\nb\nc\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unique sorted with duplicates exits 1",
			Args:      []string{"-c", "-u"},
			Stdin:     []byte("a\na\nb\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:     "check empty input",
			Args:     []string{"-c"},
			Stdin:    []byte(""),
			ExitCode: 0,
		},
		{
			Name:     "check single line",
			Args:     []string{"-c"},
			Stdin:    []byte("only\n"),
			ExitCode: 0,
		},
		// R4.2: -c check with sort modes
		{
			Name:     "check sorted human numeric",
			Args:     []string{"-c", "-h"},
			Stdin:    []byte("100\n1K\n1M\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unsorted human numeric exits 1",
			Args:      []string{"-c", "-h"},
			Stdin:     []byte("1M\n1K\n100\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:     "check sorted month",
			Args:     []string{"-c", "-M"},
			Stdin:    []byte("JAN\nFEB\nMAR\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unsorted month exits 1",
			Args:      []string{"-c", "-M"},
			Stdin:     []byte("MAR\nJAN\nFEB\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:     "check sorted version",
			Args:     []string{"-c", "-V"},
			Stdin:    []byte("file1\nfile2\nfile10\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unsorted version exits 1",
			Args:      []string{"-c", "-V"},
			Stdin:     []byte("file10\nfile2\nfile1\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:     "check sorted with key field",
			Args:     []string{"-c", "-t:", "-k2,2"},
			Stdin:    []byte("b:1\na:2\nc:3\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unsorted with key field exits 1",
			Args:      []string{"-c", "-t:", "-k2,2"},
			Stdin:     []byte("a:3\nb:1\nc:2\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:     "check quiet long form unsorted exits 1 no stderr",
			Args:     []string{"--check=quiet"},
			Stdin:    []byte("b\na\nc\n"),
			ExitCode: 1,
		},
		{
			Name:     "check quiet sorted exits 0",
			Args:     []string{"-C"},
			Stdin:    []byte("a\nb\nc\n"),
			ExitCode: 0,
		},
		{
			Name:     "check with ignore blanks sorted",
			Args:     []string{"-c", "-b"},
			Stdin:    []byte("  apple\nbanana\n  cherry\n"),
			ExitCode: 0,
		},
		// R4.2: -c combined with numeric+reverse
		{
			Name:     "check sorted numeric reverse",
			Args:     []string{"-c", "-n", "-r"},
			Stdin:    []byte("10\n2\n1\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unsorted numeric reverse exits 1",
			Args:      []string{"-c", "-n", "-r"},
			Stdin:     []byte("1\n2\n10\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.3: -c and -C are incompatible, exit 2
		{
			Name:      "check and check quiet incompatible exits 2",
			Args:      []string{"-c", "-C"},
			Stdin:     []byte("b\na\n"),
			ExitCode:  2,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: -c check with key+numeric
		{
			Name:     "check sorted key numeric",
			Args:     []string{"-c", "-t:", "-k2,2n"},
			Stdin:    []byte("b:1\na:2\nc:10\n"),
			ExitCode: 0,
		},
		{
			Name:      "check unsorted key numeric exits 1",
			Args:      []string{"-c", "-t:", "-k2,2n"},
			Stdin:     []byte("a:10\nb:2\nc:1\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		// R4.2: -c check with version sort
		{
			Name:     "check sorted version ascending",
			Args:     []string{"-c", "-V"},
			Stdin:    []byte("v1.1\nv1.2\nv1.10\n"),
			ExitCode: 0,
		},
		// R4.2: --check=silent long form
		{
			Name:     "check silent long form unsorted exits 1",
			Args:     []string{"--check=silent"},
			Stdin:    []byte("b\na\nc\n"),
			ExitCode: 1,
		},
		// R4.3: exit 2 on conflicting options (tested via TestUsageError)

		// R4.4: additional comprehensive differential tests
		{
			Name:  "default sort stdin",
			Args:  []string{},
			Stdin: []byte("cherry\napple\nbanana\n"),
		},
		{
			Name:  "reverse sort",
			Args:  []string{"-r"},
			Stdin: []byte("apple\nbanana\ncherry\n"),
		},
		{
			Name:  "single line input",
			Args:  []string{},
			Stdin: []byte("only\n"),
		},
		{
			Name:  "empty input no crash",
			Args:  []string{},
			Stdin: []byte(""),
		},
		{
			Name:  "numeric and reverse combined",
			Args:  []string{"-n", "-r"},
			Stdin: []byte("1\n10\n2\n"),
		},
		{
			Name:  "stable with numeric sort",
			Args:  []string{"-s", "-n"},
			Stdin: []byte("2 b\n1 c\n2 a\n1 b\n"),
		},
		{
			Name:  "unique with key field",
			Args:  []string{"-u", "-t:", "-k1,1"},
			Stdin: []byte("a:1\na:2\nb:1\nb:2\n"),
		},
		{
			Name:  "key with per-key reverse",
			Args:  []string{"-t:", "-k2,2r"},
			Stdin: []byte("a:1\nb:3\nc:2\n"),
		},
		{
			Name:  "multiple keys with mixed modes",
			Args:  []string{"-t:", "-k1,1", "-k2,2n"},
			Stdin: []byte("a:10\nb:2\na:3\nb:1\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffOutputFile tests -o FILE flag (R1.6) by verifying the output
// file content matches the reference binary.
func TestDiffOutputFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skip("reference binary gsort not in PATH")
	}

	t.Run("output to file", func(t *testing.T) {
		dir := t.TempDir()
		inputFile := filepath.Join(dir, "input.txt")
		goOutFile := filepath.Join(dir, "go_out.txt")
		refOutFile := filepath.Join(dir, "ref_out.txt")
		input := []byte("banana\napple\ncherry\n")
		if err := os.WriteFile(inputFile, input, 0o644); err != nil {
			t.Fatal(err)
		}

		// Run reference binary
		refCmd := exec.Command(refBin, "-o", refOutFile, inputFile)
		refCmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
		if out, err := refCmd.CombinedOutput(); err != nil {
			t.Fatalf("ref binary failed: %v\n%s", err, out)
		}
		refOutput, err := os.ReadFile(refOutFile)
		if err != nil {
			t.Fatal(err)
		}

		// Run Go binary
		goCmd := exec.Command(goBin, "-o", goOutFile, inputFile)
		goCmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
		if out, err := goCmd.CombinedOutput(); err != nil {
			t.Fatalf("go binary failed: %v\n%s", err, out)
		}
		goOutput, err := os.ReadFile(goOutFile)
		if err != nil {
			t.Fatal(err)
		}

		if string(refOutput) != string(goOutput) {
			t.Errorf("output file mismatch\nexpected (ref): %q\nactual   (go):  %q",
				refOutput, goOutput)
		}
	})

	t.Run("output to same file as input", func(t *testing.T) {
		dir := t.TempDir()
		inputFile := filepath.Join(dir, "data.txt")
		input := []byte("cherry\napple\nbanana\n")
		if err := os.WriteFile(inputFile, input, 0o644); err != nil {
			t.Fatal(err)
		}

		// Run Go binary with -o pointing to same file
		goCmd := exec.Command(goBin, "-o", inputFile, inputFile)
		goCmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
		if out, err := goCmd.CombinedOutput(); err != nil {
			t.Fatalf("go binary failed: %v\n%s", err, out)
		}

		goOutput, err := os.ReadFile(inputFile)
		if err != nil {
			t.Fatal(err)
		}

		expected := "apple\nbanana\ncherry\n"
		if string(goOutput) != expected {
			t.Errorf("output file mismatch\nexpected: %q\nactual:   %q",
				expected, goOutput)
		}
	})
}

// TestDiffMultiFile tests multi-file input (R1.3, R4.4).
func TestDiffMultiFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skip("reference binary gsort not in PATH")
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "f1.txt")
	file2 := filepath.Join(dir, "f2.txt")
	if err := os.WriteFile(file1, []byte("cherry\napple\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("banana\ndate\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:    "multi-file combined sort",
			Args:    []string{file1, file2},
			WorkDir: dir,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffCheckFile tests -c check mode with file arguments (R4.2, R4.4).
func TestDiffCheckFile(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gsort")
	if err != nil {
		t.Skip("reference binary gsort not in PATH")
	}

	dir := t.TempDir()
	sortedFile := filepath.Join(dir, "sorted.txt")
	unsortedFile := filepath.Join(dir, "unsorted.txt")
	if err := os.WriteFile(sortedFile, []byte("a\nb\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unsortedFile, []byte("b\na\nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []testutils.DiffTest{
		{
			Name:     "check sorted file exits 0",
			Args:     []string{"-c", sortedFile},
			WorkDir:  dir,
			ExitCode: 0,
		},
		{
			Name:      "check unsorted file exits 1",
			Args:      []string{"-c", unsortedFile},
			WorkDir:   dir,
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normalizeProgramName},
		},
		{
			Name:     "check quiet unsorted file exits 1",
			Args:     []string{"-C", unsortedFile},
			WorkDir:  dir,
			ExitCode: 1,
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestUsageError verifies that invalid flags produce exit code 2 (R4.3).
func TestUsageError(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")

	tests := []struct {
		name string
		args []string
	}{
		{"invalid long flag", []string{"--invalid-flag"}},
		{"invalid short flag", []string{"-Q"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(goBin, tc.args...)
			cmd.Env = append([]string{"LC_ALL=C"}, os.Environ()...)
			err := cmd.Run()
			if err == nil {
				t.Fatal("expected non-zero exit code")
			}
			exitErr, ok := err.(*exec.ExitError)
			if !ok {
				t.Fatalf("expected *exec.ExitError, got %T", err)
			}
			if exitErr.ExitCode() != 2 {
				t.Errorf("expected exit code 2, got %d", exitErr.ExitCode())
			}
		})
	}
}
