// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd086-hashutil R2.1–R2.2: ParseChecksumLine for checksum
// line parsing in GNU and BSD tag formats.
package hashutil

import (
	"fmt"
	"regexp"
	"strings"
)

// bsdTagPattern matches BSD-tag format: "ALGORITHM (FILENAME) = DIGEST".
var bsdTagPattern = regexp.MustCompile(`^(\S+) \((.+)\) = ([0-9a-fA-F]+)$`)

// gnuPattern matches GNU format: "DIGEST  FILENAME" or "DIGEST *FILENAME".
var gnuPattern = regexp.MustCompile(`^([0-9a-fA-F]+) ([ *])(.+)$`)

// ParseChecksumLine parses a single checksum line in either GNU or BSD-tag
// format and returns the filename, expected digest, binary flag, and any error.
//
// R2.1: auto-detects format by checking BSD-tag pattern first.
func ParseChecksumLine(line string, cfg HashConfig) (filename, expectedDigest string, binary bool, err error) {
	if m := bsdTagPattern.FindStringSubmatch(line); m != nil {
		return parseBSDTag(m, cfg)
	}
	if m := gnuPattern.FindStringSubmatch(line); m != nil {
		return parseGNU(m, cfg)
	}
	return "", "", false, fmt.Errorf("malformed checksum line")
}

// parseBSDTag extracts fields from a BSD-tag format match.
func parseBSDTag(m []string, cfg HashConfig) (string, string, bool, error) {
	digest := strings.ToLower(m[3])
	if err := validateDigest(digest, cfg); err != nil {
		return "", "", false, err
	}
	return m[2], digest, false, nil
}

// parseGNU extracts fields from a GNU format match.
func parseGNU(m []string, cfg HashConfig) (string, string, bool, error) {
	digest := strings.ToLower(m[1])
	if err := validateDigest(digest, cfg); err != nil {
		return "", "", false, err
	}
	return m[3], digest, m[2] == "*", nil
}

// validateDigest checks that the digest has the expected length.
func validateDigest(digest string, cfg HashConfig) error {
	if cfg.DigestLen > 0 && len(digest) != cfg.DigestLen {
		return fmt.Errorf("unexpected digest length %d (expected %d)", len(digest), cfg.DigestLen)
	}
	return nil
}
