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

// writeTestFile creates a file with the given content in a temp directory.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing test file %s: %v", name, err)
	}
	return path
}

func TestComputeDigest(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	// MD5 of "hello\n" is b1946ac92492d2347c6235b4d2611184
	r := strings.NewReader("hello\n")
	digest, err := ComputeDigest(r, cfg)
	if err != nil {
		t.Fatalf("ComputeDigest error: %v", err)
	}
	want := "b1946ac92492d2347c6235b4d2611184"
	if digest != want {
		t.Errorf("ComputeDigest = %q, want %q", digest, want)
	}
}

func TestFormatGNU(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		digest   string
		filename string
		binary   bool
		want     string
	}{
		{"text mode", "abc123", "file.txt", false, "abc123  file.txt"},
		{"binary mode", "abc123", "file.txt", true, "abc123 *file.txt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := FormatGNU(tc.digest, tc.filename, tc.binary)
			if got != tc.want {
				t.Errorf("FormatGNU = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatBSDTag(t *testing.T) {
	t.Parallel()
	got := FormatBSDTag("MD5", "file.txt", "abc123")
	want := "MD5 (file.txt) = abc123"
	if got != want {
		t.Errorf("FormatBSDTag = %q, want %q", got, want)
	}
}

func TestParseChecksumLine(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	tests := []struct {
		name           string
		line           string
		wantFile       string
		wantDigest     string
		wantBinary     bool
		wantErr        bool
	}{
		{
			name:       "GNU text mode",
			line:       "b1946ac92492d2347c6235b4d2611184  hello.txt",
			wantFile:   "hello.txt",
			wantDigest: "b1946ac92492d2347c6235b4d2611184",
			wantBinary: false,
		},
		{
			name:       "GNU binary mode",
			line:       "b1946ac92492d2347c6235b4d2611184 *hello.txt",
			wantFile:   "hello.txt",
			wantDigest: "b1946ac92492d2347c6235b4d2611184",
			wantBinary: true,
		},
		{
			name:       "BSD tag format",
			line:       "MD5 (hello.txt) = b1946ac92492d2347c6235b4d2611184",
			wantFile:   "hello.txt",
			wantDigest: "b1946ac92492d2347c6235b4d2611184",
			wantBinary: false,
		},
		{
			name:    "malformed line",
			line:    "not a checksum line",
			wantErr: true,
		},
		{
			name:    "wrong digest length",
			line:    "abcd  file.txt",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			filename, digest, binary, err := ParseChecksumLine(tc.line, cfg)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if filename != tc.wantFile {
				t.Errorf("filename = %q, want %q", filename, tc.wantFile)
			}
			if digest != tc.wantDigest {
				t.Errorf("digest = %q, want %q", digest, tc.wantDigest)
			}
			if binary != tc.wantBinary {
				t.Errorf("binary = %v, want %v", binary, tc.wantBinary)
			}
		})
	}
}

func TestVerifyChecksums_AllMatch(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	dir := t.TempDir()

	// Create a test file
	writeTestFile(t, dir, "hello.txt", "hello\n")

	// Create a checksum file with the correct digest
	checksumContent := "b1946ac92492d2347c6235b4d2611184  " + filepath.Join(dir, "hello.txt") + "\n"
	checksumFile := writeTestFile(t, dir, "checksums.md5", checksumContent)

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, cfg, CheckOptions{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("VerifyChecksums error: %v", err)
	}
	if !ok {
		t.Errorf("VerifyChecksums returned false, want true")
	}
	if !strings.Contains(stdout.String(), "OK") {
		t.Errorf("stdout should contain OK, got %q", stdout.String())
	}
}

func TestVerifyChecksums_Mismatch(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	dir := t.TempDir()

	writeTestFile(t, dir, "hello.txt", "hello\n")

	// Wrong digest
	checksumContent := "00000000000000000000000000000000  " + filepath.Join(dir, "hello.txt") + "\n"
	checksumFile := writeTestFile(t, dir, "checksums.md5", checksumContent)

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, cfg, CheckOptions{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("VerifyChecksums error: %v", err)
	}
	if ok {
		t.Errorf("VerifyChecksums returned true, want false")
	}
	if !strings.Contains(stdout.String(), "FAILED") {
		t.Errorf("stdout should contain FAILED, got %q", stdout.String())
	}
}

func TestVerifyChecksums_Quiet(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	dir := t.TempDir()

	writeTestFile(t, dir, "hello.txt", "hello\n")

	checksumContent := "b1946ac92492d2347c6235b4d2611184  " + filepath.Join(dir, "hello.txt") + "\n"
	checksumFile := writeTestFile(t, dir, "checksums.md5", checksumContent)

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, cfg, CheckOptions{Quiet: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("VerifyChecksums error: %v", err)
	}
	if !ok {
		t.Errorf("VerifyChecksums returned false, want true")
	}
	// Quiet suppresses OK lines
	if stdout.String() != "" {
		t.Errorf("stdout should be empty with Quiet, got %q", stdout.String())
	}
}

func TestVerifyChecksums_Status(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	dir := t.TempDir()

	writeTestFile(t, dir, "hello.txt", "hello\n")

	// Wrong digest — Status suppresses all output
	checksumContent := "00000000000000000000000000000000  " + filepath.Join(dir, "hello.txt") + "\n"
	checksumFile := writeTestFile(t, dir, "checksums.md5", checksumContent)

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, cfg, CheckOptions{Status: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("VerifyChecksums error: %v", err)
	}
	if ok {
		t.Errorf("VerifyChecksums returned true, want false")
	}
	if stdout.String() != "" {
		t.Errorf("stdout should be empty with Status, got %q", stdout.String())
	}
}

func TestVerifyChecksums_Warn(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	dir := t.TempDir()

	// Checksum file with a malformed line
	checksumContent := "not a valid checksum line\n"
	checksumFile := writeTestFile(t, dir, "checksums.md5", checksumContent)

	var stdout, stderr bytes.Buffer
	_, err := VerifyChecksums(checksumFile, cfg, CheckOptions{Warn: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("VerifyChecksums error: %v", err)
	}
	if !strings.Contains(stderr.String(), "no properly formatted") {
		t.Errorf("stderr should contain warning, got %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "improperly formatted") {
		t.Errorf("stderr should contain malformed summary, got %q", stderr.String())
	}
}

func TestDigestFiles_MultipleFiles(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	dir := t.TempDir()

	f1 := writeTestFile(t, dir, "a.txt", "hello\n")
	f2 := writeTestFile(t, dir, "b.txt", "world\n")

	var stdout, stderr bytes.Buffer
	exitCode := DigestFiles([]string{f1, f2}, cfg, false, false, &stdout, &stderr)
	if exitCode != 0 {
		t.Errorf("DigestFiles exit code = %d, want 0", exitCode)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d: %q", len(lines), stdout.String())
	}
	// Each line should be in GNU text format
	for _, line := range lines {
		if !strings.Contains(line, "  ") {
			t.Errorf("expected GNU text format (two spaces), got %q", line)
		}
	}
}

func TestDigestFiles_BinaryMode(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	dir := t.TempDir()

	f := writeTestFile(t, dir, "a.txt", "hello\n")

	var stdout, stderr bytes.Buffer
	exitCode := DigestFiles([]string{f}, cfg, true, false, &stdout, &stderr)
	if exitCode != 0 {
		t.Errorf("DigestFiles exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), " *") {
		t.Errorf("expected binary mode marker ' *', got %q", stdout.String())
	}
}

func TestDigestFiles_TagMode(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	dir := t.TempDir()

	f := writeTestFile(t, dir, "a.txt", "hello\n")

	var stdout, stderr bytes.Buffer
	exitCode := DigestFiles([]string{f}, cfg, false, true, &stdout, &stderr)
	if exitCode != 0 {
		t.Errorf("DigestFiles exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), "MD5 (") {
		t.Errorf("expected BSD tag format, got %q", stdout.String())
	}
}

func TestDigestFiles_MissingFile(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	dir := t.TempDir()

	f1 := writeTestFile(t, dir, "a.txt", "hello\n")
	missing := filepath.Join(dir, "nonexistent.txt")

	var stdout, stderr bytes.Buffer
	exitCode := DigestFiles([]string{f1, missing}, cfg, false, false, &stdout, &stderr)
	if exitCode != 1 {
		t.Errorf("DigestFiles exit code = %d, want 1", exitCode)
	}
	// Should still have output for the first file
	if !strings.Contains(stdout.String(), "b1946ac92492d2347c6235b4d2611184") {
		t.Errorf("stdout should contain digest for a.txt, got %q", stdout.String())
	}
	// Should have error for missing file
	if !strings.Contains(stderr.String(), "nonexistent.txt") {
		t.Errorf("stderr should mention missing file, got %q", stderr.String())
	}
}

func TestDigestFiles_Stdin(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()

	// Create a pipe to simulate stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}

	// Save and restore os.Stdin
	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	// Write data and close the write end
	go func() {
		w.Write([]byte("hello\n"))
		w.Close() // best-effort close
	}()

	var stdout, stderr bytes.Buffer
	exitCode := DigestFiles([]string{"-"}, cfg, false, false, &stdout, &stderr)
	if exitCode != 0 {
		t.Errorf("DigestFiles exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), "b1946ac92492d2347c6235b4d2611184") {
		t.Errorf("stdout should contain MD5 of 'hello\\n', got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "-") {
		t.Errorf("stdout should use '-' as filename, got %q", stdout.String())
	}
}

func TestDigestFiles_EmptyList(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()

	// Create a pipe to simulate stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}

	oldStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = oldStdin })

	go func() {
		w.Write([]byte("hello\n"))
		w.Close() // best-effort close
	}()

	var stdout, stderr bytes.Buffer
	exitCode := DigestFiles([]string{}, cfg, false, false, &stdout, &stderr)
	if exitCode != 0 {
		t.Errorf("DigestFiles exit code = %d, want 0", exitCode)
	}
	if !strings.Contains(stdout.String(), "b1946ac92492d2347c6235b4d2611184") {
		t.Errorf("stdout should contain digest from stdin, got %q", stdout.String())
	}
}

func TestComputeDigest_EmptyInput(t *testing.T) {
	t.Parallel()
	cfg := testMD5Config()
	r := strings.NewReader("")
	digest, err := ComputeDigest(r, cfg)
	if err != nil {
		t.Fatalf("ComputeDigest error: %v", err)
	}
	// MD5 of empty string is d41d8cd98f00b204e9800998ecf8427e
	want := "d41d8cd98f00b204e9800998ecf8427e"
	if digest != want {
		t.Errorf("ComputeDigest = %q, want %q", digest, want)
	}
}

func TestParseChecksumLine_NoDigestLenCheck(t *testing.T) {
	t.Parallel()
	// When DigestLen is 0, any length digest should be accepted
	cfg := HashConfig{
		Algorithm: "TEST",
		NewHash:   func() hash.Hash { return md5.New() },
		DigestLen: 0,
	}
	filename, digest, _, err := ParseChecksumLine("abcd  file.txt", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filename != "file.txt" {
		t.Errorf("filename = %q, want %q", filename, "file.txt")
	}
	if digest != "abcd" {
		t.Errorf("digest = %q, want %q", digest, "abcd")
	}
}
