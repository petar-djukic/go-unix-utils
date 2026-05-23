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
	refBin, err := exec.LookPath("gbasenc")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tmpDir := t.TempDir()
	helloFile := filepath.Join(tmpDir, "hello.txt")
	writeFile(t, helloFile, "hello\n")
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	writeFile(t, emptyFile, "")
	z85File := filepath.Join(tmpDir, "z85.dat")
	writeFile(t, z85File, "test")

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R1.1: --base64 encode
		{Name: "base64-encode", Args: []string{"--base64"}, Stdin: []byte("hello\n"), Env: []string{"LC_ALL=C"}},
		// R1.2: --base64url encode
		{Name: "base64url-encode", Args: []string{"--base64url"}, Stdin: []byte("hello\n"), Env: []string{"LC_ALL=C"}},
		// R1.3: --base32 encode
		{Name: "base32-encode", Args: []string{"--base32"}, Stdin: []byte("hello\n"), Env: []string{"LC_ALL=C"}},
		// R1.4: --base32hex encode
		{Name: "base32hex-encode", Args: []string{"--base32hex"}, Stdin: []byte("hello\n"), Env: []string{"LC_ALL=C"}},
		// R2.1: --base16 encode
		{Name: "base16-encode", Args: []string{"--base16"}, Stdin: []byte("hello\n"), Env: []string{"LC_ALL=C"}},
		{Name: "base16-encode-binary", Args: []string{"--base16"}, Stdin: []byte("\x00\x01\x02\xff\xfe"), Env: []string{"LC_ALL=C"}},
		{Name: "base16-encode-empty", Args: []string{"--base16"}, Stdin: []byte(""), Env: []string{"LC_ALL=C"}},
		// R2.2: --z85 encode
		{Name: "z85-encode-4bytes", Args: []string{"--z85"}, Stdin: []byte("test"), Env: []string{"LC_ALL=C"}},
		{Name: "z85-encode-8bytes", Args: []string{"--z85"}, Stdin: []byte("testtest"), Env: []string{"LC_ALL=C"}},
		{Name: "z85-encode-file", Args: []string{"--z85", z85File}, Env: []string{"LC_ALL=C"}},
		{Name: "z85-encode-invalid-length", Args: []string{"--z85"}, Stdin: []byte("hello"), ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}, Env: []string{"LC_ALL=C"}},
		// R2.3: decode all alphabets
		{Name: "base64-decode", Args: []string{"--base64", "-d"}, Stdin: []byte("aGVsbG8K\n"), Env: []string{"LC_ALL=C"}},
		{Name: "base64url-decode", Args: []string{"--base64url", "-d"}, Stdin: []byte("aGVsbG8K\n"), Env: []string{"LC_ALL=C"}},
		{Name: "base32-decode", Args: []string{"--base32", "-d"}, Stdin: []byte("NBSWY3DPBI======\n"), Env: []string{"LC_ALL=C"}},
		{Name: "base32hex-decode", Args: []string{"--base32hex", "-d"}, Stdin: []byte("D1IMOR3F18======\n"), Env: []string{"LC_ALL=C"}},
		{Name: "base16-decode", Args: []string{"--base16", "-d"}, Stdin: []byte("68656C6C6F0A\n"), Env: []string{"LC_ALL=C"}},
		{Name: "z85-decode", Args: []string{"--z85", "-d"}, Stdin: []byte("HelloWorld\n"), Env: []string{"LC_ALL=C"}},
		{Name: "decode-long-flag", Args: []string{"--base64", "--decode"}, Stdin: []byte("aGVsbG8K\n"), Env: []string{"LC_ALL=C"}},
		// R2.3: decode strips whitespace
		{Name: "decode-strips-whitespace", Args: []string{"--base64", "-d"}, Stdin: []byte("aGVs\nbG8K\n"), Env: []string{"LC_ALL=C"}},
		// R2.4: wrap column
		{Name: "wrap-default-76", Args: []string{"--base64"}, Stdin: []byte("The quick brown fox jumps over the lazy dog. The quick brown fox jumps over the lazy dog."), Env: []string{"LC_ALL=C"}},
		{Name: "wrap-40", Args: []string{"--base64", "-w", "40"}, Stdin: []byte("The quick brown fox jumps over the lazy dog."), Env: []string{"LC_ALL=C"}},
		{Name: "wrap-0", Args: []string{"--base64", "-w", "0"}, Stdin: []byte("The quick brown fox jumps over the lazy dog."), Env: []string{"LC_ALL=C"}},
		{Name: "wrap-long-form", Args: []string{"--base64", "--wrap=20"}, Stdin: []byte("hello world"), Env: []string{"LC_ALL=C"}},
		{Name: "wrap-short-attached", Args: []string{"--base64", "-w0"}, Stdin: []byte("hello"), Env: []string{"LC_ALL=C"}},
		// R3.1: --ignore-garbage
		{Name: "ignore-garbage-base64", Args: []string{"--base64", "-d", "-i"}, Stdin: []byte("aGVs!!!bG8K\n"), Env: []string{"LC_ALL=C"}},
		{Name: "ignore-garbage-long-form", Args: []string{"--base64", "--decode", "--ignore-garbage"}, Stdin: []byte("aGVs!!!bG8K\n"), Env: []string{"LC_ALL=C"}},
		{Name: "ignore-garbage-base32", Args: []string{"--base32", "-d", "-i"}, Stdin: []byte("NBSWY3DP!!!BI======\n"), Env: []string{"LC_ALL=C"}},
		// R3.2: file and stdin input
		{Name: "encode-file", Args: []string{"--base64", helloFile}, Env: []string{"LC_ALL=C"}},
		{Name: "encode-empty-file", Args: []string{"--base64", emptyFile}, Env: []string{"LC_ALL=C"}},
		{Name: "stdin-dash", Args: []string{"--base64", "-"}, Stdin: []byte("hello\n"), Env: []string{"LC_ALL=C"}},
		{Name: "encode-empty-stdin", Args: []string{"--base64"}, Stdin: []byte(""), Env: []string{"LC_ALL=C"}},
		// R3.3: error cases
		{Name: "no-encoding", Stdin: []byte("hello\n"), ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}, Env: []string{"LC_ALL=C"}},
		{Name: "missing-file", Args: []string{"--base64", filepath.Join(tmpDir, "nonexistent.txt")}, ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}, Env: []string{"LC_ALL=C"}},
		{Name: "decode-invalid-input", Args: []string{"--base64", "-d"}, Stdin: []byte("!!!invalid!!!\n"), ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}, Env: []string{"LC_ALL=C"}},
		{Name: "z85-decode-invalid", Args: []string{"--z85", "-d"}, Stdin: []byte("abc\n"), ExitCode: 1, Normalize: []testutils.NormalizeFunc{discardStderr}, Env: []string{"LC_ALL=C"}},
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, "--base64", largePath)
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
			t.Fatalf("timed out; SIGPIPE handler may not be installed")
		}
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
