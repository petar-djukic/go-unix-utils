// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildBinary compiles the comm binary and returns its path.
func buildBinary(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	bin := filepath.Join(binDir, "comm")
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build comm: %v", err)
	}
	return bin
}

type commTest struct {
	name  string
	file1 string
	file2 string
	args  []string
	stdin string // non-empty when one arg is "-"
}

func TestDiff(t *testing.T) {
	goBin := buildBinary(t)
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	tests := []commTest{
		{name: "three_column_output", file1: "a\nb\nc\n", file2: "b\nc\nd\n"},
		{name: "suppress_col1", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"-1"}},
		{name: "suppress_col2", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"-2"}},
		{name: "suppress_col3", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"-3"}},
		{name: "common_only_12", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"-12"}},
		{name: "suppress_all_123", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"-123"}},
		{name: "output_delimiter", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"--output-delimiter=|"}},
		{name: "empty_file1", file1: "", file2: "a\nb\n"},
		{name: "empty_file2", file1: "a\nb\n", file2: ""},
		{name: "both_empty", file1: "", file2: ""},
		{name: "identical_files", file1: "a\nb\nc\n", file2: "a\nb\nc\n"},
		{name: "no_common_lines", file1: "a\nc\ne\n", file2: "b\nd\nf\n"},
		{name: "nocheck_order", file1: "c\na\n", file2: "b\nd\n", args: []string{"--nocheck-order"}},
		{name: "total", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"--total"}},
		{name: "total_with_suppress", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"-1", "--total"}},
		{name: "file1_exhausted_first", file1: "a\n", file2: "a\nb\n"},
		{name: "file2_exhausted_first", file1: "a\nb\n", file2: "a\n"},
		{name: "single_line_common", file1: "x\n", file2: "x\n"},
		{name: "single_line_different", file1: "a\n", file2: "b\n"},
		{name: "combined_23", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"-23"}},
		{name: "combined_13", file1: "a\nb\nc\n", file2: "b\nc\nd\n", args: []string{"-13"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()

			f1 := filepath.Join(dir, "file1.txt")
			f2 := filepath.Join(dir, "file2.txt")
			if err := os.WriteFile(f1, []byte(tc.file1), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(f2, []byte(tc.file2), 0o644); err != nil {
				t.Fatal(err)
			}

			args := make([]string, 0, len(tc.args)+2)
			args = append(args, tc.args...)
			args = append(args, f1, f2)

			goStdout, goStderr, goExit := runBin(t, goBin, args, tc.stdin)
			refStdout, refStderr, refExit := runBin(t, refBin, args, tc.stdin)

			if goExit != refExit {
				t.Errorf("exit code: go=%d ref=%d", goExit, refExit)
			}
			if !bytes.Equal(goStdout, refStdout) {
				t.Errorf("stdout mismatch:\n  go:  %q\n  ref: %q", goStdout, refStdout)
			}
			if !bytes.Equal(goStderr, refStderr) {
				t.Errorf("stderr mismatch:\n  go:  %q\n  ref: %q", goStderr, refStderr)
			}
		})
	}
}

func TestDiffStdin(t *testing.T) {
	goBin := buildBinary(t)
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	f2 := filepath.Join(dir, "file2.txt")
	if err := os.WriteFile(f2, []byte("b\nc\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdinData := "a\nb\nc\n"
	args := []string{"-", f2}

	goStdout, goStderr, goExit := runBin(t, goBin, args, stdinData)
	refStdout, refStderr, refExit := runBin(t, refBin, args, stdinData)

	if goExit != refExit {
		t.Errorf("exit code: go=%d ref=%d", goExit, refExit)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\n  go:  %q\n  ref: %q", goStdout, refStdout)
	}
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\n  go:  %q\n  ref: %q", goStderr, refStderr)
	}
}

func TestDiffCheckOrder(t *testing.T) {
	goBin := buildBinary(t)
	refBin, err := exec.LookPath("gcomm")
	if err != nil {
		t.Skipf("reference binary gcomm not in PATH: %v", err)
	}

	dir := t.TempDir()
	f1 := filepath.Join(dir, "file1.txt")
	f2 := filepath.Join(dir, "file2.txt")
	if err := os.WriteFile(f1, []byte("c\na\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("b\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	args := []string{"--check-order", f1, f2}

	goStdout, goStderr, goExit := runBin(t, goBin, args, "")
	refStdout, refStderr, refExit := runBin(t, refBin, args, "")

	if goExit != refExit {
		t.Errorf("exit code: go=%d ref=%d", goExit, refExit)
	}
	if !bytes.Equal(goStdout, refStdout) {
		t.Errorf("stdout mismatch:\n  go:  %q\n  ref: %q", goStdout, refStdout)
	}
	if !bytes.Equal(goStderr, refStderr) {
		t.Errorf("stderr mismatch:\n  go:  %q\n  ref: %q", goStderr, refStderr)
	}
}

func runBin(t *testing.T, bin string, args []string, stdin string) (stdout, stderr []byte, exitCode int) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if stdin != "" {
		cmd.Stdin = bytes.NewReader([]byte(stdin))
	}
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return outBuf.Bytes(), errBuf.Bytes(), exitCode
}
