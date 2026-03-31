// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package hashutil

import (
	"bytes"
	"crypto/md5"
	"hash"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testMD5Config returns a HashConfig for MD5 used across tests.
func testMD5Config() HashConfig {
	return HashConfig{
		Algorithm: "MD5",
		NewHash:   md5.New,
		DigestLen: 32,
	}
}

// TestDigestFilesContinuesOnError verifies R3.2: DigestFiles continues
// processing remaining files when one file cannot be opened, printing
// an error to stderr for each failure.
func TestDigestFilesContinuesOnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create two valid files and reference a missing one in between.
	validFile1 := filepath.Join(dir, "a.txt")
	validFile2 := filepath.Join(dir, "c.txt")
	missingFile := filepath.Join(dir, "b-missing.txt")

	writeTestFile(t, validFile1, "hello\n")
	writeTestFile(t, validFile2, "world\n")

	var stdout, stderr bytes.Buffer
	cfg := testMD5Config()
	exitCode := DigestFiles(
		[]string{validFile1, missingFile, validFile2},
		cfg, false, false, &stdout, &stderr,
	)

	// R3.2: exit code is 1 because one file failed.
	if exitCode != 1 {
		t.Errorf("expected exit code 1, got %d", exitCode)
	}

	// R3.2: stderr contains error for the missing file.
	if !strings.Contains(stderr.String(), "b-missing.txt") {
		t.Errorf("stderr should mention missing file, got: %s", stderr.String())
	}

	// R3.2: stdout contains digests for both valid files.
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 digest lines, got %d: %v", len(lines), lines)
	}
}

// TestDigestFilesAllValid verifies R3.1: successful processing of
// multiple files returns exit code 0.
func TestDigestFilesAllValid(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	file1 := filepath.Join(dir, "x.txt")
	writeTestFile(t, file1, "data\n")

	var stdout, stderr bytes.Buffer
	cfg := testMD5Config()
	exitCode := DigestFiles(
		[]string{file1},
		cfg, false, false, &stdout, &stderr,
	)

	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	if stderr.Len() != 0 {
		t.Errorf("expected empty stderr, got: %s", stderr.String())
	}
	if !strings.Contains(stdout.String(), file1) {
		t.Errorf("stdout should contain filename, got: %s", stdout.String())
	}
}

// TestDigestFilesStdin verifies R3.1: stdin fallback when files list
// is empty.
func TestDigestFilesStdin(t *testing.T) {
	t.Parallel()

	// DigestFiles with empty list defaults to stdin ("-").
	// We cannot easily inject stdin in this test, so verify the
	// function at least attempts to read from "-".
	var stdout, stderr bytes.Buffer
	cfg := testMD5Config()

	// Provide "-" explicitly; stdin in test context is /dev/null-like,
	// so it should produce the hash of empty input.
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = r
	w.Close() // EOF immediately
	defer func() { os.Stdin = oldStdin }()

	exitCode := DigestFiles([]string{"-"}, cfg, false, false, &stdout, &stderr)
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d", exitCode)
	}
	// MD5 of empty input: d41d8cd98f00b204e9800998ecf8427e
	if !strings.Contains(stdout.String(), "d41d8cd98f00b204e9800998ecf8427e") {
		t.Errorf("expected MD5 of empty input, got: %s", stdout.String())
	}
}

// TestComputeDigest verifies R1.4: digest computation returns correct
// hex-encoded value.
func TestComputeDigest(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	r := strings.NewReader("hello\n")
	digest, err := ComputeDigest(r, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// MD5 of "hello\n" is b1946ac92492d2347c6235b4d2611184
	if digest != "b1946ac92492d2347c6235b4d2611184" {
		t.Errorf("expected b1946ac92492d2347c6235b4d2611184, got %s", digest)
	}
}

// TestFormatGNU verifies R1.2: GNU text and binary format output.
func TestFormatGNU(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		binary bool
		want   string
	}{
		{"text mode", false, "abc123  file.txt"},
		{"binary mode", true, "abc123 *file.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FormatGNU("abc123", "file.txt", tc.binary)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFormatBSDTag verifies R1.3: BSD tag format output.
func TestFormatBSDTag(t *testing.T) {
	t.Parallel()
	got := FormatBSDTag("MD5", "file.txt", "abc123")
	want := "MD5 (file.txt) = abc123"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestParseChecksumLine verifies R2.1: parsing GNU and BSD tag formats.
func TestParseChecksumLine(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()

	tests := []struct {
		name       string
		line       string
		wantFile   string
		wantDigest string
		wantBinary bool
		wantErr    bool
	}{
		{
			name:       "GNU text mode",
			line:       "d41d8cd98f00b204e9800998ecf8427e  empty.txt",
			wantFile:   "empty.txt",
			wantDigest: "d41d8cd98f00b204e9800998ecf8427e",
			wantBinary: false,
		},
		{
			name:       "GNU binary mode",
			line:       "d41d8cd98f00b204e9800998ecf8427e *binary.dat",
			wantFile:   "binary.dat",
			wantDigest: "d41d8cd98f00b204e9800998ecf8427e",
			wantBinary: true,
		},
		{
			name:       "BSD tag format",
			line:       "MD5 (file.txt) = d41d8cd98f00b204e9800998ecf8427e",
			wantFile:   "file.txt",
			wantDigest: "d41d8cd98f00b204e9800998ecf8427e",
			wantBinary: false,
		},
		{
			name:    "malformed line",
			line:    "not a valid checksum line",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, digest, binary, err := ParseChecksumLine(tc.line, cfg)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if file != tc.wantFile {
				t.Errorf("filename: got %q, want %q", file, tc.wantFile)
			}
			if digest != tc.wantDigest {
				t.Errorf("digest: got %q, want %q", digest, tc.wantDigest)
			}
			if binary != tc.wantBinary {
				t.Errorf("binary: got %v, want %v", binary, tc.wantBinary)
			}
		})
	}
}

// TestVerifyChecksums verifies R2.3: checksum verification with
// pass and fail cases.
func TestVerifyChecksums(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := testMD5Config()

	// Create a file with known content.
	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "hello\n")

	// Create a valid checksum file.
	checksumFile := filepath.Join(dir, "checksums.md5")
	// MD5 of "hello\n" = b1946ac92492d2347c6235b4d2611184
	writeTestFile(t, checksumFile,
		"b1946ac92492d2347c6235b4d2611184  "+dataFile+"\n")

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, cfg, CheckOptions{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected verification to pass")
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("expected OK in output, got: %s", stdout.String())
	}
}

// TestVerifyChecksumsFailed verifies R2.3: returns false when digest
// does not match.
func TestVerifyChecksumsFailed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := testMD5Config()

	dataFile := filepath.Join(dir, "data.txt")
	writeTestFile(t, dataFile, "hello\n")

	checksumFile := filepath.Join(dir, "bad.md5")
	writeTestFile(t, checksumFile,
		"0000000000000000000000000000dead  "+dataFile+"\n")

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, cfg, CheckOptions{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected verification to fail")
	}
	if !strings.Contains(stdout.String(), "FAILED") {
		t.Errorf("expected FAILED in output, got: %s", stdout.String())
	}
}

// writeTestFile is a test helper that writes content to a file.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// fakeMD5Hash is unused but ensures hash import is used.
var _ hash.Hash = md5.New()
