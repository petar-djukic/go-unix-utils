// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package hashutil provides shared hash output formatting and checksum
// verification logic for md5sum, sha1sum, sha256sum, sha512sum, sha224sum,
// sha384sum, and b2sum utilities. Implements srd086-hashutil.
package hashutil

import (
	"hash"
	"io"
)

// HashConfig parameterizes a hash utility with its algorithm name, hash
// factory, and expected digest length.
// R1.1: Algorithm identifies the hash (e.g. "MD5", "SHA256").
// R1.1: NewHash creates a new hash.Hash instance for computing digests.
// R1.1: DigestLen is the expected hex digest length (e.g. 32 for MD5).
type HashConfig struct {
	Algorithm string
	NewHash   func() hash.Hash
	DigestLen int
}

// CheckOptions controls --check mode output behavior.
// R2.2: Warn prints warnings for malformed lines.
// R2.2: Quiet suppresses OK lines.
// R2.2: Status suppresses all output, reporting only via exit code.
type CheckOptions struct {
	Warn   bool
	Quiet  bool
	Status bool
}

// FormatGNU formats a digest and filename in GNU coreutils format.
// R1.2: returns "HASH  FILENAME" (text) or "HASH *FILENAME" (binary).
func FormatGNU(digest, filename string, binary bool) string {
	return ""
}

// FormatBSDTag formats a digest in BSD tag format.
// R1.3: returns "ALGORITHM (FILENAME) = HASH".
func FormatBSDTag(algorithm, filename, digest string) string {
	return ""
}

// ComputeDigest reads all bytes from r, computes the hash using cfg, and
// returns the lowercase hex-encoded digest string.
// R1.4: delegates to cfg.NewHash for the hash function.
func ComputeDigest(r io.Reader, cfg HashConfig) (string, error) {
	return "", nil
}

// ParseChecksumLine parses a single checksum line in GNU or BSD tag format.
// R2.1: returns the filename, expected digest, binary mode flag, and any error.
func ParseChecksumLine(line string, cfg HashConfig) (filename, expectedDigest string, binary bool, err error) {
	return "", "", false, nil
}

// VerifyChecksums reads a checksum file and verifies each entry against the
// actual file contents.
// R2.3: prints results according to opts and returns whether all checks passed.
func VerifyChecksums(checksumFile string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	return false, nil
}

// DigestFiles computes and prints digests for each file in the list.
// R3.1: reads from stdin when files is empty or contains "-".
// R3.2: continues processing remaining files when one cannot be opened.
func DigestFiles(files []string, cfg HashConfig, binary, tag bool, stdout, stderr io.Writer) int {
	return 0
}
