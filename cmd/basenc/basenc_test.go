// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Differential tests for cmd/basenc: srd081 R1.1-R1.4, R2.1-R2.4, R3.1-R3.4.
package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/petar-djukic/go-unix-utils/pkg/testutils"
)

// TestDiffBase64 tests --base64 encode/decode against gbasenc.
// Traces: srd081 R1.1 (base64), R2.3 (decode), R2.4 (wrap).
func TestDiffBase64(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	tests := []testutils.DiffTest{
		{
			Name:  "base64 encode short",
			Args:  []string{"--base64"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "base64 encode empty",
			Args:  []string{"--base64"},
			Stdin: []byte(""),
		},
		{
			Name:  "base64 encode binary data",
			Args:  []string{"--base64"},
			Stdin: []byte{0x00, 0x01, 0x02, 0xff, 0xfe, 0xfd},
		},
		{
			Name:  "base64 decode",
			Args:  []string{"--base64", "-d"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		{
			Name:  "base64 decode padded",
			Args:  []string{"--base64", "-d"},
			Stdin: []byte("YQ==\n"),
		},
		{
			Name:  "base64 wrap default 76",
			Args:  []string{"--base64"},
			Stdin: bytes.Repeat([]byte("A"), 200),
		},
		{
			Name:  "base64 wrap 0 disables",
			Args:  []string{"--base64", "-w", "0"},
			Stdin: bytes.Repeat([]byte("B"), 200),
		},
		{
			Name:  "base64 wrap 40",
			Args:  []string{"--base64", "-w", "40"},
			Stdin: bytes.Repeat([]byte("C"), 200),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBase64URL tests --base64url encode/decode against gbasenc.
// Traces: srd081 R1.2 (base64url).
func TestDiffBase64URL(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	tests := []testutils.DiffTest{
		{
			Name:  "base64url encode short",
			Args:  []string{"--base64url"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "base64url encode binary with special chars",
			Args:  []string{"--base64url"},
			Stdin: []byte{0xfb, 0xff, 0xfe},
		},
		{
			Name:  "base64url decode",
			Args:  []string{"--base64url", "-d"},
			Stdin: []byte("aGVsbG8K\n"),
		},
		{
			Name:  "base64url encode empty",
			Args:  []string{"--base64url"},
			Stdin: []byte(""),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBase32 tests --base32 encode/decode against gbasenc.
// Traces: srd081 R1.3 (base32).
func TestDiffBase32(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	tests := []testutils.DiffTest{
		{
			Name:  "base32 encode short",
			Args:  []string{"--base32"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "base32 encode single byte",
			Args:  []string{"--base32"},
			Stdin: []byte("A"),
		},
		{
			Name:  "base32 decode",
			Args:  []string{"--base32", "-d"},
			Stdin: []byte("NBSWY3DPBI======\n"),
		},
		{
			Name:  "base32 encode empty",
			Args:  []string{"--base32"},
			Stdin: []byte(""),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBase32Hex tests --base32hex encode/decode against gbasenc.
// Traces: srd081 R1.4 (base32hex).
func TestDiffBase32Hex(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	tests := []testutils.DiffTest{
		{
			Name:  "base32hex encode short",
			Args:  []string{"--base32hex"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "base32hex decode",
			Args:  []string{"--base32hex", "-d"},
			Stdin: []byte("D1IMOR3F18======\n"),
		},
		{
			Name:  "base32hex encode empty",
			Args:  []string{"--base32hex"},
			Stdin: []byte(""),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffBase16 tests --base16 encode/decode against gbasenc.
// Traces: srd081 R2.1 (base16 hex encoding).
func TestDiffBase16(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	tests := []testutils.DiffTest{
		{
			Name:  "base16 encode short",
			Args:  []string{"--base16"},
			Stdin: []byte("hello\n"),
		},
		{
			Name:  "base16 encode binary",
			Args:  []string{"--base16"},
			Stdin: []byte{0x00, 0xff, 0x0a},
		},
		{
			Name:  "base16 decode",
			Args:  []string{"--base16", "-d"},
			Stdin: []byte("68656C6C6F0A\n"),
		},
		{
			Name:  "base16 encode empty",
			Args:  []string{"--base16"},
			Stdin: []byte(""),
		},
		{
			Name:  "base16 wrap 0",
			Args:  []string{"--base16", "-w", "0"},
			Stdin: bytes.Repeat([]byte("X"), 100),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffZ85 tests --z85 encode/decode against gbasenc.
// Traces: srd081 R2.2 (z85 encoding).
func TestDiffZ85(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	tests := []testutils.DiffTest{
		{
			Name:  "z85 encode 4 bytes",
			Args:  []string{"--z85"},
			Stdin: []byte{0x86, 0x4f, 0xd2, 0x6f},
		},
		{
			Name:  "z85 encode 8 bytes",
			Args:  []string{"--z85"},
			Stdin: []byte{0x86, 0x4f, 0xd2, 0x6f, 0xb5, 0x59, 0xf7, 0x5b},
		},
		{
			Name:  "z85 decode",
			Args:  []string{"--z85", "-d"},
			Stdin: []byte("HelloWorld\n"),
		},
		{
			Name:  "z85 encode empty",
			Args:  []string{"--z85"},
			Stdin: []byte(""),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDecodeIgnoreGarbage tests -i flag across alphabets.
// Traces: srd081 R3.1 (ignore-garbage).
func TestDiffDecodeIgnoreGarbage(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	tests := []testutils.DiffTest{
		{
			Name:  "base64 ignore-garbage",
			Args:  []string{"--base64", "-d", "-i"},
			Stdin: []byte("aGVs!!!bG8K\n"),
		},
		{
			Name:  "base64 ignore-garbage long flag",
			Args:  []string{"--base64", "-d", "--ignore-garbage"},
			Stdin: []byte("aGVs@#$bG8K\n"),
		},
		{
			Name:  "base32 ignore-garbage",
			Args:  []string{"--base32", "-d", "-i"},
			Stdin: []byte("NBSWY3DP!!!BI======\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffErrors tests error reporting.
// Traces: srd081 R3.3 (exit 1 on no encoding, file error, invalid decode).
func TestDiffErrors(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	tests := []testutils.DiffTest{
		// R3.3: no encoding scheme exits 1.
		{
			Name:      "no encoding flag exits 1",
			Stdin:     []byte("hello\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		// R3.3: missing file exits 1.
		{
			Name:      "missing file exits 1",
			Args:      []string{"--base64", "nonexistent_file_xyz"},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		// R3.3: invalid decode input exits 1.
		{
			Name:      "base64 invalid decode exits 1",
			Args:      []string{"--base64", "-d"},
			Stdin:     []byte("!!!invalid!!!\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "base16 invalid decode exits 1",
			Args:      []string{"--base16", "-d"},
			Stdin:     []byte("ZZZZ\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
		{
			Name:      "base32 invalid decode exits 1",
			Args:      []string{"--base32", "-d"},
			Stdin:     []byte("!!!invalid!!!\n"),
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffFileInput tests encoding from a file argument.
// Traces: srd081 R3.2 (read from FILE).
func TestDiffFileInput(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	dir := t.TempDir()
	inputFile := filepath.Join(dir, "input.txt")
	writeTestFile(t, inputFile, "file content\n")

	tests := []testutils.DiffTest{
		{
			Name: "base64 encode from file",
			Args: []string{"--base64", inputFile},
		},
		{
			Name:  "base64 encode from stdin dash",
			Args:  []string{"--base64", "-"},
			Stdin: []byte("hello\n"),
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffDecodeFile tests decoding from a file argument.
// Traces: srd081 R3.2 (read from FILE), R2.3 (decode).
func TestDiffDecodeFile(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	dir := t.TempDir()
	inputFile := filepath.Join(dir, "encoded.txt")
	writeTestFile(t, inputFile, "aGVsbG8K\n")

	tests := []testutils.DiffTest{
		{
			Name: "base64 decode from file",
			Args: []string{"--base64", "-d", inputFile},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffPermissionError tests file permission error reporting.
// Traces: srd081 R3.3 (exit 1 on file error).
func TestDiffPermissionError(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	goBin, refBin := buildAndLookup(t)

	dir := t.TempDir()
	noReadFile := filepath.Join(dir, "noperm.txt")
	if err := os.WriteFile(noReadFile, []byte("data"), 0o000); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	tests := []testutils.DiffTest{
		{
			Name:      "permission denied exits 1",
			Args:      []string{"--base64", noReadFile},
			ExitCode:  1,
			Normalize: []testutils.NormalizeFunc{normStderr},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// TestDiffSpecialFlags tests --help and --version flags.
// Traces: srd081 general flag handling.
func TestDiffSpecialFlags(t *testing.T) {
	t.Parallel()
	goBin, refBin := buildAndLookup(t)

	tests := []testutils.DiffTest{
		{
			Name:      "version flag exits 0",
			Args:      []string{"--version"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
		{
			Name:      "help flag exits 0",
			Args:      []string{"--help"},
			Normalize: []testutils.NormalizeFunc{clearOutput},
		},
	}

	testutils.RunDiffTests(t, goBin, refBin, tests)
}

// buildAndLookup builds the Go binary and looks up the reference binary.
func buildAndLookup(t *testing.T) (string, string) {
	t.Helper()
	goBin := testutils.BuildBinary(t, ".")
	refBin, err := exec.LookPath("gbasenc")
	if err != nil {
		t.Skip("reference binary gbasenc not in PATH")
	}
	return goBin, refBin
}

// writeTestFile writes content to a file, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", path, err)
	}
}

// normStderr normalizes error output so gbasenc and our binary produce
// comparable stderr. Handles program name, OS error case, and drops
// "Try" lines (binary paths differ).
func normStderr(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var out []byte
	for _, line := range lines {
		line = bytes.ReplaceAll(line, []byte("gbasenc: "), []byte("basenc: "))
		line = bytes.ReplaceAll(line, []byte("No such file"), []byte("no such file"))
		line = bytes.ReplaceAll(line, []byte("Permission denied"), []byte("permission denied"))
		if bytes.HasPrefix(line, []byte("Try '")) {
			continue
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return out
}

// clearOutput clears all output for tests where only exit code matters.
func clearOutput(data []byte) []byte {
	return nil
}
