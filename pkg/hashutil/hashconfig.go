// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package hashutil provides shared hash output formatting and checksum
// verification logic for md5sum, sha1sum, sha256sum, sha512sum, sha224sum,
// sha384sum, b2sum, cksum, and sum utilities.
// Implements srd086-hashutil R1.1, R1.2, R1.3, R1.4.
package hashutil

import (
	"hash"
	"io"
)

// HashConfig parameterizes a hash utility with its algorithm name, hash
// factory, and expected digest length. (R1.1)
type HashConfig struct {
	Algorithm string
	NewHash   func() hash.Hash
	DigestLen int
}

// CheckOptions controls --check mode output behavior. (R1.2 / R2.2)
type CheckOptions struct {
	Warn   bool
	Quiet  bool
	Status bool
}

// FormatGNU formats a digest and filename in GNU coreutils format.
// Text mode: "HASH  FILENAME"; binary mode: "HASH *FILENAME". (R1.3)
func FormatGNU(digest, filename string, binary bool) string {
	return ""
}

// FormatBSDTag formats a digest in BSD tag format:
// "ALGORITHM (FILENAME) = HASH". (R1.3)
func FormatBSDTag(algorithm, filename, digest string) string {
	return ""
}

// ComputeDigest reads all bytes from r, computes the hash using cfg, and
// returns the lowercase hex-encoded digest string. (R1.4)
func ComputeDigest(r io.Reader, cfg HashConfig) (string, error) {
	return "", nil
}

// ParseChecksumLine parses a single checksum line in GNU or BSD tag format.
// (R1.4 / R2.1)
func ParseChecksumLine(line string, cfg HashConfig) (filename, expectedDigest string, binary bool, err error) {
	return "", "", false, nil
}

// VerifyChecksums reads a checksum file and verifies each entry against the
// actual file contents. (R1.4 / R2.3)
func VerifyChecksums(checksumFile string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	return false, nil
}

// DigestFiles computes and prints digests for each file in the list. (R1.4 / R3.1)
func DigestFiles(files []string, cfg HashConfig, binary, tag bool, stdout, stderr io.Writer) int {
	return 0
}
