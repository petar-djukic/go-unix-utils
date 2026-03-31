// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package hashutil provides shared hash output formatting and checksum
// verification logic for cmd/ hash utilities (md5sum, sha1sum, sha256sum,
// sha512sum, sha224sum, sha384sum, b2sum, cksum, sum).
//
// Implements prd086-hashutil: R1 (hash output formatting), R2 (checksum
// verification), R3 (file processing).
package hashutil

import "hash"

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
	Strict bool // exit non-zero for malformed checksum lines
}
