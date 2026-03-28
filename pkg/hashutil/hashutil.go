// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package hashutil provides shared hash output formatting and verification
// logic for checksum utilities (md5sum, sha1sum, sha256sum, etc.).
//
// Implements prd086-hashutil R1.1–R1.4: HashConfig, CheckOptions, FormatGNU,
// FormatBSDTag, ComputeDigest.
package hashutil

import (
	"encoding/hex"
	"fmt"
	"hash"
	"io"
)

// HashConfig parameterizes a hash utility with the algorithm name,
// hash constructor, and expected digest length.
//
// R1.1: Algorithm identifies the hash (e.g. "MD5", "SHA256").
// NewHash is a factory for the hash function. DigestLen is the
// expected hex digest length (e.g. 32 for MD5, 64 for SHA-256).
type HashConfig struct {
	Algorithm string
	NewHash   func() hash.Hash
	DigestLen int
}

// CheckOptions controls --check mode behavior for checksum verification.
//
// R1.2 (task R2.2 in PRD): Warn prints warnings for malformed lines,
// Quiet suppresses OK lines, Status suppresses all output.
type CheckOptions struct {
	Warn   bool
	Quiet  bool
	Status bool
}

// FormatGNU formats a digest and filename in GNU checksum format.
//
// R1.2: text mode produces "HASH  FILENAME" (two spaces).
// Binary mode produces "HASH *FILENAME" (space-asterisk).
func FormatGNU(digest, filename string, binary bool) string {
	sep := "  "
	if binary {
		sep = " *"
	}
	return digest + sep + filename
}

// FormatBSDTag formats a digest and filename in BSD tag style.
//
// R1.3: produces "ALGORITHM (FILENAME) = DIGEST".
func FormatBSDTag(algorithm, filename, digest string) string {
	return fmt.Sprintf("%s (%s) = %s", algorithm, filename, digest)
}

// ComputeDigest reads all bytes from r, computes the hash using
// cfg.NewHash(), and returns the lowercase hex-encoded digest string.
//
// R1.4: does not close the reader. Returns an error if reading fails.
func ComputeDigest(r io.Reader, cfg HashConfig) (string, error) {
	h := cfg.NewHash()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("computing %s digest: %w", cfg.Algorithm, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
