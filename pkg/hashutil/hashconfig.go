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
type HashConfig struct {
	Algorithm string
	NewHash   func() hash.Hash
	DigestLen int
}

// CheckOptions controls --check mode output behavior.
type CheckOptions struct {
	Warn   bool
	Quiet  bool
	Status bool
}

// FormatGNU formats a digest and filename in GNU coreutils format.
func FormatGNU(digest, filename string, binary bool) string {
	panic("not implemented")
}

// FormatBSDTag formats a digest in BSD tag format.
func FormatBSDTag(algorithm, filename, digest string) string {
	panic("not implemented")
}

// ComputeDigest reads all bytes from r, computes the hash using cfg, and
// returns the lowercase hex-encoded digest string.
func ComputeDigest(r io.Reader, cfg HashConfig) (string, error) {
	panic("not implemented")
}

// ParseChecksumLine parses a single checksum line in GNU or BSD tag format.
func ParseChecksumLine(line string, cfg HashConfig) (filename, expectedDigest string, binary bool, err error) {
	panic("not implemented")
}

// VerifyChecksums reads a checksum file and verifies each entry against the
// actual file contents.
func VerifyChecksums(checksumFile string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	panic("not implemented")
}

// DigestFiles computes and prints digests for each file in the list.
func DigestFiles(files []string, cfg HashConfig, binary, tag bool, stdout, stderr io.Writer) int {
	panic("not implemented")
}
