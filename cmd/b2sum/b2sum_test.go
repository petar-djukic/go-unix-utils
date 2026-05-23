// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/blake2b"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

func TestDiff(t *testing.T) {
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gb2sum")
	if err != nil {
		t.Skip("reference binary not found")
	}

	tmpDir := t.TempDir()
	helloFile := filepath.Join(tmpDir, "hello.txt")
	writeFile(t, helloFile, "hello\n")
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	writeFile(t, emptyFile, "")

	discardStderr := testutils.NormalizeFunc(func([]byte) []byte { return nil })

	hello512 := b2hex([]byte("hello\n"), 64)
	empty512 := b2hex([]byte{}, 64)
	hello256 := b2hex([]byte("hello\n"), 32)
	zeroHash := strings.Repeat("0", 128)

	validChecksum := filepath.Join(tmpDir, "valid.b2")
	writeFile(t, validChecksum, fmt.Sprintf("%s  %s\n", hello512, helloFile))

	multiChecksum := filepath.Join(tmpDir, "multi.b2")
	writeFile(t, multiChecksum, fmt.Sprintf("%s  %s\n%s  %s\n", hello512, helloFile, empty512, emptyFile))

	mismatchChecksum := filepath.Join(tmpDir, "mismatch.b2")
	writeFile(t, mismatchChecksum, fmt.Sprintf("%s  %s\n", zeroHash, helloFile))

	bsdChecksum := filepath.Join(tmpDir, "bsd.b2")
	writeFile(t, bsdChecksum, fmt.Sprintf("BLAKE2b (%s) = %s\n", helloFile, hello512))

	mixedChecksum := filepath.Join(tmpDir, "mixed.b2")
	writeFile(t, mixedChecksum, fmt.Sprintf("%s  %s\n%s  %s\n", hello512, helloFile, zeroHash, emptyFile))

	malformedChecksum := filepath.Join(tmpDir, "malformed.b2")
	writeFile(t, malformedChecksum, fmt.Sprintf("this is not a checksum\n%s  %s\n", hello512, helloFile))

	len256Checksum := filepath.Join(tmpDir, "len256.b2")
	writeFile(t, len256Checksum, fmt.Sprintf("%s  %s\n", hello256, helloFile))

	bsd256Checksum := filepath.Join(tmpDir, "bsd256.b2")
	writeFile(t, bsd256Checksum, fmt.Sprintf("BLAKE2b-256 (%s) = %s\n", helloFile, hello256))

	tests := []testutils.DiffTest{
		// R1.1: single file, text mode (default)
		{
			Name: "single-file-text",
			Args: []string{helloFile},
		},
		// R1.1: single file, binary mode
		{
			Name: "single-file-binary",
			Args: []string{"-b", helloFile},
		},
		// R1.2: stdin with no arguments
		{
			Name:  "stdin-no-args",
			Stdin: []byte("abc"),
		},
		// R1.2: stdin with explicit "-"
		{
			Name:  "stdin-dash",
			Args:  []string{"-"},
			Stdin: []byte("abc"),
		},
		// R1.3: BSD tag format
		{
			Name: "tag-format",
			Args: []string{"--tag", helloFile},
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
		// R2.1: --check with valid checksum file
		{
			Name:      "check-valid",
			Args:      []string{"--check", validChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.1: --check with BSD tag format
		{
			Name:      "check-bsd-tag",
			Args:      []string{"--check", bsdChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.2: --check with mismatched hash exits 1
		{
			Name:      "check-mismatch",
			Args:      []string{"--check", mismatchChecksum},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.3: --check --warn with malformed lines
		{
			Name:      "check-warn-malformed",
			Args:      []string{"--check", "--warn", malformedChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.3: --check --quiet suppresses OK lines
		{
			Name:      "check-quiet-all-ok",
			Args:      []string{"--check", "--quiet", multiChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.3: --check --status suppresses all output
		{
			Name:      "check-status-ok",
			Args:      []string{"--check", "--status", validChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R2.3: --check --status exits 1 on failure
		{
			Name:      "check-status-fail",
			Args:      []string{"--check", "--status", mismatchChecksum},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},

		// R3.1: binary mode with multiple files
		{
			Name: "binary-multiple-files",
			Args: []string{"-b", helloFile, emptyFile},
		},
		// R3.1: text mode explicit
		{
			Name: "text-mode-explicit",
			Args: []string{"-t", helloFile},
		},

		// R3.2: --tag produces BSD tag format regardless of -b
		{
			Name: "tag-overrides-binary",
			Args: []string{"--tag", "-b", helloFile},
		},
		// R3.2: --tag rejects explicit text mode
		{
			Name:      "tag-rejects-text",
			Args:      []string{"--tag", "-t", helloFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.2: --tag with -b and multiple files
		{
			Name: "tag-binary-multiple",
			Args: []string{"--tag", "-b", helloFile, emptyFile},
		},

		// R3.3: --length=256 produces 64-char hex digest
		{
			Name: "length-256",
			Args: []string{"--length=256", helloFile},
		},
		// R3.3: --length 256 (space-separated)
		{
			Name: "length-256-space",
			Args: []string{"--length", "256", helloFile},
		},
		// R3.3: --length=256 with --tag uses BLAKE2b-256
		{
			Name: "length-256-tag",
			Args: []string{"--length=256", "--tag", helloFile},
		},
		// R3.3: --length=128
		{
			Name: "length-128",
			Args: []string{"--length=128", helloFile},
		},
		// R3.3: --length=512 (explicit default)
		{
			Name: "length-512-explicit",
			Args: []string{"--length=512", helloFile},
		},
		// R3.3: invalid length (not multiple of 8) exits 1
		{
			Name:      "length-invalid-not-mult-8",
			Args:      []string{"--length=7", helloFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.3: invalid length (zero) exits 1
		{
			Name:      "length-invalid-zero",
			Args:      []string{"--length=0", helloFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.3: invalid length (exceeds 512) exits 1
		{
			Name:      "length-invalid-too-large",
			Args:      []string{"--length=520", helloFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.3: invalid length (negative) exits 1
		{
			Name:      "length-invalid-negative",
			Args:      []string{"--length=-8", helloFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.3: --length=256 with --check
		{
			Name:      "length-256-check",
			Args:      []string{"--length=256", "--check", len256Checksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R3.3: --length=256 with --check BSD tag format
		{
			Name:      "length-256-check-bsd",
			Args:      []string{"--length=256", "--check", bsd256Checksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},

		// R4.1: exit 0 when all files processed successfully
		{
			Name: "exit-0-all-ok",
			Args: []string{helloFile, emptyFile},
		},
		// R4.1: exit 0 when all verified digests match
		{
			Name:      "exit-0-check-all-match",
			Args:      []string{"--check", multiChecksum},
			ExitCode:  0,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},

		// R4.2: exit 1 when any file cannot be opened
		{
			Name:      "exit-1-missing-file",
			Args:      []string{filepath.Join(tmpDir, "no-such-file.txt")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2: exit 1 when any digest fails verification
		{
			Name:      "exit-1-check-fail",
			Args:      []string{"--check", mismatchChecksum},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2: exit 1 when checksum file is unreadable
		{
			Name:      "exit-1-check-unreadable",
			Args:      []string{"--check", filepath.Join(tmpDir, "nonexistent.b2")},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{discardStderr},
		},
		// R4.2: exit 1 mixed valid/invalid in check
		{
			Name:      "exit-1-check-mixed",
			Args:      []string{"--check", mixedChecksum},
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
		t.Fatal("b2sum timed out; SIGPIPE handler may not be installed")
	}
	if err != nil {
		t.Fatalf("expected exit 0 on SIGPIPE, got: %v", err)
	}
}

func b2hex(data []byte, size int) string {
	h, _ := blake2b.New(size, nil)
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
