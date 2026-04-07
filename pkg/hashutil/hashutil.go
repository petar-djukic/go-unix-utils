// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package hashutil provides shared hash output formatting and checksum
// verification logic used by cmd/ hash utilities (md5sum, sha1sum, etc.).
//
// Implements: srd086-hashutil R1 (hash output formatting), R2 (checksum
// verification), R3 (file processing).
package hashutil

import (
	"hash"
	"io"
)

// HashConfig parameterizes a hash utility with its algorithm name, hash
// factory, and expected digest length.
// R1.1: Algorithm is e.g. "MD5", "SHA256", "BLAKE2b"; NewHash creates
// the hash.Hash instance; DigestLen is the expected hex digest length.
type HashConfig struct {
	Algorithm string
	NewHash   func() hash.Hash
	DigestLen int
}

// CheckOptions controls --check mode behavior for checksum verification.
// R2.2: Warn prints warnings for malformed lines; Quiet suppresses OK
// lines; Status suppresses all output and reports only via exit code.
type CheckOptions struct {
	Warn   bool
	Quiet  bool
	Status bool
}

// FormatGNU returns a GNU-format checksum line: "HASH  FILENAME" for text
// mode or "HASH *FILENAME" for binary mode.
// R1.2: two-space separator for text mode, space-asterisk for binary mode.
func FormatGNU(digest, filename string, binary bool) string {
	panic("hashutil.FormatGNU: not yet implemented")
}

// FormatBSDTag returns a BSD tag-format checksum line:
// "ALGORITHM (FILENAME) = HASH".
// R1.3: uses the algorithm name, parenthesized filename, and digest.
func FormatBSDTag(algorithm, filename, digest string) string {
	panic("hashutil.FormatBSDTag: not yet implemented")
}

// ComputeDigest reads all bytes from r, computes the hash using cfg, and
// returns the lowercase hex-encoded digest string.
// R1.4: delegates to cfg.NewHash for the hash instance.
func ComputeDigest(r io.Reader, cfg HashConfig) (string, error) {
	panic("hashutil.ComputeDigest: not yet implemented")
}

// ParseChecksumLine parses a single checksum line in GNU format
// ("HASH [ *]FILENAME") or BSD tag format ("ALGORITHM (FILENAME) = HASH").
// Returns the filename, expected digest, binary flag, and an error for
// malformed lines.
// R2.1: supports both GNU and BSD tag format detection.
func ParseChecksumLine(line string, cfg HashConfig) (filename, expectedDigest string, binary bool, err error) {
	panic("hashutil.ParseChecksumLine: not yet implemented")
}

// VerifyChecksums reads a checksum file, verifies each entry against the
// actual file contents, prints results according to opts, and returns
// whether all checks passed.
// R2.3: respects CheckOptions for output control.
func VerifyChecksums(checksumFile string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	panic("hashutil.VerifyChecksums: not yet implemented")
}

// DigestFiles computes and prints digests for each file in files (or stdin
// when files is empty or contains "-"). Returns exit code 0 for success,
// 1 if any file failed.
// R3.1: formats output as GNU or BSD tag depending on the tag flag.
// R3.2: continues processing remaining files on error.
func DigestFiles(files []string, cfg HashConfig, binary, tag bool, stdout, stderr io.Writer) int {
	panic("hashutil.DigestFiles: not yet implemented")
}
