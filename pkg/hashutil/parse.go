// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd086-hashutil R2.1–R2.3: ParseChecksumLine and
// VerifyChecksums for checksum file verification.
package hashutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
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

// VerifyChecksums reads a checksum file, verifies each entry against the
// actual file contents, and prints results according to opts.
//
// R2.3: returns true if all checks passed, false otherwise.
// Prints "FILENAME: OK" or "FILENAME: FAILED" to stdout.
// Prints warnings for malformed lines to stderr when opts.Warn is set.
func VerifyChecksums(checksumFile string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	f, err := openChecksumFile(checksumFile)
	if err != nil {
		return false, err
	}
	defer f.Close() // best-effort close on read-only file

	return processChecksumLines(f, cfg, opts, stdout, stderr)
}

// openChecksumFile opens the checksum file, or returns stdin for "-".
func openChecksumFile(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// processChecksumLines iterates over lines and verifies each checksum.
func processChecksumLines(r io.Reader, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	scanner := bufio.NewScanner(r)
	allOK := true
	malformed := 0

	for scanner.Scan() {
		line := scanner.Text()
		ok, err := verifySingleLine(line, cfg, opts, stdout, stderr)
		if err != nil {
			malformed++
			continue
		}
		if !ok {
			allOK = false
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("reading checksum file: %w", err)
	}
	reportMalformed(malformed, stderr)
	return allOK, nil
}

// verifySingleLine parses and verifies one checksum line.
// Returns (true, nil) if digest matches, (false, nil) if it doesn't,
// or ("", "", err) if the line is malformed.
func verifySingleLine(line string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	filename, expected, _, err := ParseChecksumLine(line, cfg)
	if err != nil {
		if opts.Warn {
			fmt.Fprintf(stderr, "%s: no properly formatted checksum lines found\n", line)
		}
		return true, err
	}
	return verifyFile(filename, expected, cfg, opts, stdout, stderr), nil
}

// verifyFile computes the digest of a file and compares it to expected.
func verifyFile(filename, expected string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) bool {
	actual, err := computeFileDigest(filename, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", filename, err)
		reportResult(filename, false, opts, stdout)
		return false
	}
	match := actual == expected
	reportResult(filename, match, opts, stdout)
	return match
}

// computeFileDigest opens a file and computes its hash digest.
func computeFileDigest(filename string, cfg HashConfig) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close() // best-effort close on read-only file
	return ComputeDigest(f, cfg)
}

// reportResult prints the verification result according to opts.
func reportResult(filename string, ok bool, opts CheckOptions, stdout io.Writer) {
	if opts.Status {
		return
	}
	if ok && opts.Quiet {
		return
	}
	status := "OK"
	if !ok {
		status = "FAILED"
	}
	fmt.Fprintf(stdout, "%s: %s\n", filename, status)
}

// reportMalformed prints a summary warning for malformed lines.
func reportMalformed(count int, stderr io.Writer) {
	if count > 0 {
		fmt.Fprintf(stderr, "WARNING: %d line is improperly formatted\n", count)
	}
}
