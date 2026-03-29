// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package hashutil provides shared hash output formatting and checksum
// verification logic for cmd/ hash utilities (md5sum, sha1sum, sha256sum,
// sha512sum, sha224sum, sha384sum, b2sum, cksum, sum).
//
// Implements prd086-hashutil: R1 (hash output formatting), R2 (checksum
// verification), R3 (file processing).
package hashutil

import (
	"hash"
	"io"
)

// HashConfig parameterizes a hash utility with the algorithm name, hash
// constructor, and expected digest length.
//
// R1.1: HashConfig struct definition.
type HashConfig struct {
	Algorithm string          // e.g. "MD5", "SHA256", "BLAKE2b"
	NewHash   func() hash.Hash // factory for the hash function
	DigestLen int             // expected hex digest length (e.g., 32 for MD5, 64 for SHA-256)
}

// CheckOptions controls --check mode behavior for checksum verification.
//
// R2.2: CheckOptions struct definition.
type CheckOptions struct {
	Warn   bool // print warning for malformed lines
	Quiet  bool // suppress OK lines
	Status bool // suppress all output, report only via exit code
}

// FormatGNU formats a digest line in GNU format: "HASH  FILENAME" for text
// mode or "HASH *FILENAME" for binary mode.
//
// R1.2: GNU format output.
func FormatGNU(digest, filename string, binary bool) string {
	panic("not implemented")
}

// FormatBSDTag formats a digest line in BSD tag format:
// "ALGORITHM (FILENAME) = HASH".
//
// R1.3: BSD tag format output.
func FormatBSDTag(algorithm, filename, digest string) string {
	panic("not implemented")
}

// ComputeDigest reads all bytes from r, computes the hash using cfg, and
// returns the lowercase hex-encoded digest string.
//
// R1.4: Digest computation.
func ComputeDigest(r io.Reader, cfg HashConfig) (string, error) {
	panic("not implemented")
}

// ParseChecksumLine parses a single checksum line in GNU format
// ("HASH [ *]FILENAME") or BSD tag format ("ALGORITHM (FILENAME) = HASH").
// Returns the filename, expected digest, binary mode flag, and any parse error.
//
// R2.1: Checksum line parsing.
func ParseChecksumLine(line string, cfg HashConfig) (filename, expectedDigest string, binary bool, err error) {
	panic("not implemented")
}

// VerifyChecksums reads a checksum file, verifies each entry against the
// actual file contents, prints results according to opts, and returns whether
// all checks passed.
//
// R2.3: Checksum verification with CheckOptions.
func VerifyChecksums(checksumFile string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	panic("not implemented")
}

// DigestFiles computes and prints digests for each file (or stdin when files
// is empty or contains "-"), returning the exit code (0 for success, 1 if any
// file failed).
//
// R3.1, R3.2: File processing with error continuation.
func DigestFiles(files []string, cfg HashConfig, binary, tag bool, stdout, stderr io.Writer) int {
	panic("not implemented")
}
