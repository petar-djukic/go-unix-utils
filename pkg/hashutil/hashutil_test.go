// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package hashutil

import (
	"bytes"
	"crypto/md5"
	"hash"
	"os"
	"path/filepath"
	"testing"
)

var md5Config = HashConfig{
	Algorithm: "MD5",
	NewHash:   func() hash.Hash { return md5.New() },
	DigestLen: 32,
}

func TestParseChecksumLine(t *testing.T) {
	t.Parallel()
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
			line:       "d41d8cd98f00b204e9800998ecf8427e  empty.txt",
			wantFile:   "empty.txt",
			wantDigest: "d41d8cd98f00b204e9800998ecf8427e",
			wantBinary: false,
		},
		{
			name:       "GNU binary mode",
			line:       "d41d8cd98f00b204e9800998ecf8427e *empty.txt",
			wantFile:   "empty.txt",
			wantDigest: "d41d8cd98f00b204e9800998ecf8427e",
			wantBinary: true,
		},
		{
			name:       "BSD tag format",
			line:       "MD5 (empty.txt) = d41d8cd98f00b204e9800998ecf8427e",
			wantFile:   "empty.txt",
			wantDigest: "d41d8cd98f00b204e9800998ecf8427e",
			wantBinary: false,
		},
		{
			name:    "malformed line",
			line:    "not a valid checksum line",
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
			f, d, b, err := ParseChecksumLine(tc.line, md5Config)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f != tc.wantFile {
				t.Errorf("filename: got %q, want %q", f, tc.wantFile)
			}
			if d != tc.wantDigest {
				t.Errorf("digest: got %q, want %q", d, tc.wantDigest)
			}
			if b != tc.wantBinary {
				t.Errorf("binary: got %v, want %v", b, tc.wantBinary)
			}
		})
	}
}

func TestVerifyChecksums_AllMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a test file with known content.
	content := []byte("hello\n")
	testFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(testFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Compute its MD5 digest.
	digest, err := ComputeDigest(bytes.NewReader(content), md5Config)
	if err != nil {
		t.Fatal(err)
	}

	// Write a checksum file.
	checksumFile := filepath.Join(dir, "checksums.md5")
	line := FormatGNU(digest, testFile, false) + "\n"
	if err := os.WriteFile(checksumFile, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, md5Config, CheckOptions{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected all OK, got failure. stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if got := stdout.String(); got != testFile+": OK\n" {
		t.Errorf("stdout: got %q, want %q", got, testFile+": OK\n")
	}
}

func TestVerifyChecksums_Mismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a test file.
	testFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a checksum file with a wrong digest.
	wrongDigest := "00000000000000000000000000000000"
	checksumFile := filepath.Join(dir, "checksums.md5")
	line := FormatGNU(wrongDigest, testFile, false) + "\n"
	if err := os.WriteFile(checksumFile, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, md5Config, CheckOptions{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected failure, got ok")
	}
	if got := stdout.String(); got != testFile+": FAILED\n" {
		t.Errorf("stdout: got %q, want %q", got, testFile+": FAILED\n")
	}
}

func TestVerifyChecksums_SkipsBlanksAndComments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	testFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, err := ComputeDigest(bytes.NewReader([]byte("hello\n")), md5Config)
	if err != nil {
		t.Fatal(err)
	}

	checksumFile := filepath.Join(dir, "checksums.md5")
	content := "# comment line\n\n" + FormatGNU(digest, testFile, false) + "\n\n"
	if err := os.WriteFile(checksumFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, md5Config, CheckOptions{}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok, got failure. stderr=%q", stderr.String())
	}
}

func TestVerifyChecksums_QuietMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	testFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	digest, _ := ComputeDigest(bytes.NewReader([]byte("hello\n")), md5Config)

	checksumFile := filepath.Join(dir, "checksums.md5")
	line := FormatGNU(digest, testFile, false) + "\n"
	if err := os.WriteFile(checksumFile, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, md5Config, CheckOptions{Quiet: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok")
	}
	// Quiet suppresses OK lines.
	if stdout.String() != "" {
		t.Errorf("expected empty stdout in quiet mode, got %q", stdout.String())
	}
}

func TestVerifyChecksums_StatusMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	testFile := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(testFile, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	wrongDigest := "00000000000000000000000000000000"
	checksumFile := filepath.Join(dir, "checksums.md5")
	line := FormatGNU(wrongDigest, testFile, false) + "\n"
	if err := os.WriteFile(checksumFile, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	ok, err := VerifyChecksums(checksumFile, md5Config, CheckOptions{Status: true}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected failure")
	}
	// Status suppresses all output.
	if stdout.String() != "" {
		t.Errorf("expected empty stdout in status mode, got %q", stdout.String())
	}
	if stderr.String() != "" {
		t.Errorf("expected empty stderr in status mode, got %q", stderr.String())
	}
}
