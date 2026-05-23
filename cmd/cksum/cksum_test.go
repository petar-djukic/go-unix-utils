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
	refBin, err := exec.LookPath("gcksum")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tmpDir := t.TempDir()
	helloFile := filepath.Join(tmpDir, "hello.txt")
	if err := os.WriteFile(helloFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	binaryFile := filepath.Join(tmpDir, "binary.dat")
	if err := os.WriteFile(binaryFile, []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}, 0o644); err != nil {
		t.Fatal(err)
	}

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	tests := []testutils.DiffTest{
		// R1.1: single file produces CRC checksum and byte count
		{
			Name: "single-file",
			Args: []string{helloFile},
		},
		// R1.1: empty file
		{
			Name: "empty-file",
			Args: []string{emptyFile},
		},
		// R1.1: binary content
		{
			Name: "binary-content",
			Args: []string{binaryFile},
		},
		// R1.2: stdin with no arguments
		{
			Name:  "stdin-no-args",
			Stdin: []byte("hello\n"),
		},
		// R1.2: empty stdin
		{
			Name:  "stdin-empty",
			Stdin: []byte{},
		},
		// R1.2: stdin with explicit "-"
		{
			Name:  "stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("abc"),
		},
		// R1.3: multiple files in argument order
		{
			Name: "multiple-files",
			Args: []string{helloFile, emptyFile},
		},
		// R1.3: multiple files reversed order
		{
			Name: "multiple-files-reversed",
			Args: []string{emptyFile, helloFile},
		},
		// R1.3: three files
		{
			Name: "three-files",
			Args: []string{helloFile, emptyFile, binaryFile},
		},
		// R1.4: missing file exits 1
		{
			Name:      "missing-file",
			Args:      []string{filepath.Join(tmpDir, "nonexistent.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R1.4: missing file among valid files continues processing
		{
			Name:      "missing-among-valid",
			Args:      []string{helloFile, filepath.Join(tmpDir, "nonexistent.txt"), emptyFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.1: algorithm selection -- tagged output for each algorithm
		{
			Name: "algo-sha1",
			Args: []string{"--algorithm=sha1", helloFile},
		},
		{
			Name: "algo-sha224",
			Args: []string{"--algorithm=sha224", helloFile},
		},
		{
			Name: "algo-sha256",
			Args: []string{"--algorithm=sha256", helloFile},
		},
		{
			Name: "algo-sha384",
			Args: []string{"--algorithm=sha384", helloFile},
		},
		{
			Name: "algo-sha512",
			Args: []string{"--algorithm=sha512", helloFile},
		},
		{
			Name: "algo-blake2b",
			Args: []string{"--algorithm=blake2b", helloFile},
		},
		{
			Name: "algo-sm3",
			Args: []string{"--algorithm=sm3", helloFile},
		},
		// R2.1: explicit crc algorithm
		{
			Name: "algo-crc-explicit",
			Args: []string{"--algorithm=crc", helloFile},
		},
		// R2.1: algorithm with stdin
		{
			Name:  "algo-sha256-stdin",
			Args:  []string{"--algorithm=sha256"},
			Stdin: []byte("hello\n"),
		},
		// R2.1: algorithm with multiple files
		{
			Name: "algo-sha256-multi",
			Args: []string{"--algorithm=sha256", helloFile, emptyFile},
		},
		// R2.1: short flag -a
		{
			Name: "algo-short-space",
			Args: []string{"-a", "sha256", helloFile},
		},
		{
			Name: "algo-short-nospace",
			Args: []string{"-asha256", helloFile},
		},
		// R2.1: algorithm with empty file
		{
			Name: "algo-sha256-empty",
			Args: []string{"--algorithm=sha256", emptyFile},
		},
		// R2.1: algorithm with binary content
		{
			Name: "algo-sha256-binary",
			Args: []string{"--algorithm=sha256", binaryFile},
		},
		// R2.2: untagged output
		{
			Name: "untagged-sha256",
			Args: []string{"--algorithm=sha256", "--untagged", helloFile},
		},
		{
			Name: "untagged-sha1",
			Args: []string{"--algorithm=sha1", "--untagged", helloFile},
		},
		{
			Name: "untagged-sha256-multi",
			Args: []string{"--algorithm=sha256", "--untagged", helloFile, emptyFile},
		},
		{
			Name:  "untagged-sha256-stdin",
			Args:  []string{"--algorithm=sha256", "--untagged"},
			Stdin: []byte("hello\n"),
		},
		// R2.3: raw output
		{
			Name: "raw-sha256",
			Args: []string{"--algorithm=sha256", "--raw", helloFile},
		},
		{
			Name:  "raw-sha256-stdin",
			Args:  []string{"--algorithm=sha256", "--raw"},
			Stdin: []byte("hello\n"),
		},
		{
			Name: "raw-sm3",
			Args: []string{"--algorithm=sm3", "--raw", helloFile},
		},
		// R3.1: exit 0 on success (verified by all above tests with default ExitCode=0)
		// R3.2: invalid algorithm exits 1
		{
			Name:      "invalid-algorithm",
			Args:      []string{"--algorithm=invalid", helloFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

func TestSIGPIPE(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	dir := t.TempDir()
	largePath := filepath.Join(dir, "large.dat")
	if err := os.WriteFile(largePath, bytes.Repeat([]byte("x\n"), 500000), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, goBin, largePath)
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
	err = cmd.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatal("cksum timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}
