// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbase32")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tmpDir := t.TempDir()
	helloFile := filepath.Join(tmpDir, "hello.txt")
	writeFile(t, helloFile, "hello\n")
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	writeFile(t, emptyFile, "")
	binaryFile := filepath.Join(tmpDir, "binary.dat")
	writeFile(t, binaryFile, "\x00\x01\x02\xff\xfe")

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R1.1: encode stdin
		{
			Name:  "encode-stdin",
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: encode stdin with explicit dash
		{
			Name:  "encode-stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("hello\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.1: encode file
		{
			Name: "encode-file",
			Args: []string{helloFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: encode empty file
		{
			Name: "encode-empty",
			Args: []string{emptyFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.1: encode binary data
		{
			Name: "encode-binary",
			Args: []string{binaryFile},
			Env:  []string{"LC_ALL=C"},
		},
		// R1.2: default wrapping at 76 columns
		{
			Name:  "encode-default-wrap",
			Stdin: []byte("The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog."),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: wrap at specific column
		{
			Name:  "wrap-40",
			Args:  []string{"-w", "40"},
			Stdin: []byte("The quick brown fox jumps over the lazy dog."),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: wrap=0 disables wrapping
		{
			Name:  "wrap-0",
			Args:  []string{"-w", "0"},
			Stdin: []byte("The quick brown fox jumps over the lazy dog."),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.3: --wrap=COLS long form
		{
			Name:  "wrap-long-form",
			Args:  []string{"--wrap=20"},
			Stdin: []byte("hello world"),
			Env:   []string{"LC_ALL=C"},
		},
		// R1.4: missing file exits 1
		{
			Name:      "missing-file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
		// R2.1: decode stdin
		{
			Name:  "decode-stdin",
			Args:  []string{"-d"},
			Stdin: []byte("NBSWY3DPEB3W64TMMQ======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.1: decode with --decode
		{
			Name:  "decode-long-flag",
			Args:  []string{"--decode"},
			Stdin: []byte("NBSWY3DPEB3W64TMMQ======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.2: decode ignores whitespace
		{
			Name:  "decode-ignores-whitespace",
			Args:  []string{"-d"},
			Stdin: []byte("NBSWY3DP\nEB3W64TM\nMQ======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: decode with --ignore-garbage
		{
			Name:  "decode-ignore-garbage",
			Args:  []string{"-d", "-i"},
			Stdin: []byte("NBSWY3DP!!!EB3W64TMMQ======\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.3: decode --ignore-garbage long form
		{
			Name:  "decode-ignore-garbage-long",
			Args:  []string{"--decode", "--ignore-garbage"},
			Stdin: []byte("NBS!!WY3DP\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R2.4: invalid base32 input exits 1
		{
			Name:      "decode-invalid-input",
			Args:      []string{"-d"},
			Stdin:     []byte("!!!invalid!!!\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
		// R3.1: exit 0 on success
		{
			Name:  "exit-0-encode",
			Stdin: []byte("test"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.1: exit 0 on decode success
		{
			Name:  "exit-0-decode",
			Args:  []string{"-d"},
			Stdin: []byte("ORSXG5A=\n"),
			Env:   []string{"LC_ALL=C"},
		},
		// R3.2: exit 1 on error
		{
			Name:      "exit-1-bad-file",
			Args:      []string{filepath.Join(tmpDir, "does-not-exist")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
			Env:       []string{"LC_ALL=C"},
		},
		// Encode roundtrip
		{
			Name:  "encode-empty-stdin",
			Stdin: []byte(""),
			Env:   []string{"LC_ALL=C"},
		},
		// Short flag -w with attached value
		{
			Name:  "short-w-attached",
			Args:  []string{"-w0"},
			Stdin: []byte("hello"),
			Env:   []string{"LC_ALL=C"},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.dat")
	if err := os.WriteFile(largePath, bytes.Repeat([]byte("x"), 100000), 0o644); err != nil {
		t.Fatal(err)
	}
	testSIGPIPE(t, goBin, largePath)
}

func testSIGPIPE(t *testing.T, bin, filePath string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, filePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 1)
	if _, err := io.ReadFull(stdout, buf); err != nil {
		t.Fatal(err)
	}
	stdout.Close()
	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			t.Fatalf("%s timed out; SIGPIPE handler may not be installed", bin)
		}
		t.Fatalf("%s: expected exit 0 on SIGPIPE, got: %v", bin, err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
