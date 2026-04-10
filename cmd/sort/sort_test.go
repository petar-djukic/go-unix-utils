// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

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
