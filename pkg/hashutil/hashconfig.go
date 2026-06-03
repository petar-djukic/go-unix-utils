// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package hashutil provides shared hash output formatting and checksum
// verification logic for md5sum, sha1sum, sha256sum, sha512sum, sha224sum,
// sha384sum, b2sum, cksum, and sum utilities.
// Implements srd086-hashutil R1.1, R1.2, R1.3, R1.4.
package hashutil

import (
	"encoding/hex"
	"fmt"
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
	if binary {
		return fmt.Sprintf("%s *%s", digest, filename)
	}
	return fmt.Sprintf("%s  %s", digest, filename)
}

// FormatBSDTag formats a digest in BSD tag format:
// "ALGORITHM (FILENAME) = HASH". (R1.3)
func FormatBSDTag(algorithm, filename, digest string) string {
	return fmt.Sprintf("%s (%s) = %s", algorithm, filename, digest)
}

// ComputeDigest reads all bytes from r, computes the hash using cfg, and
// returns the lowercase hex-encoded digest string. (R1.4)
func ComputeDigest(r io.Reader, cfg HashConfig) (string, error) {
	h := cfg.NewHash()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("computing digest: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
