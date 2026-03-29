// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// verify.go implements prd086 R2.1, R2.2, R2.3:
// checksum line parsing and verification logic.

package hashutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// gnuLineRe matches GNU format: "HASH  FILENAME" or "HASH *FILENAME".
var gnuLineRe = regexp.MustCompile(`^([0-9a-fA-F]+) ([ *])(.+)$`)

// bsdTagRe matches BSD tag format: "ALGORITHM (FILENAME) = HASH".
var bsdTagRe = regexp.MustCompile(`^(\w+) \((.+)\) = ([0-9a-fA-F]+)$`)

// ParseChecksumLine parses a single checksum line in GNU format
// ("HASH [ *]FILENAME") or BSD tag format ("ALGORITHM (FILENAME) = HASH").
// Returns the filename, expected digest, binary mode flag, and any parse error.
//
// R2.1: Checksum line parsing.
func ParseChecksumLine(line string, cfg HashConfig) (filename, expectedDigest string, binary bool, err error) {
	if f, d, b, ok := parseGNULine(line, cfg); ok {
		return f, d, b, nil
	}
	if f, d, ok := parseBSDTagLine(line); ok {
		return f, d, false, nil
	}
	return "", "", false, fmt.Errorf("invalid checksum line")
}

// parseGNULine attempts to parse a GNU-format checksum line.
func parseGNULine(line string, cfg HashConfig) (filename, digest string, binary, ok bool) {
	m := gnuLineRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false, false
	}
	digest = strings.ToLower(m[1])
	if len(digest) != cfg.DigestLen {
		return "", "", false, false
	}
	return m[3], digest, m[2] == "*", true
}

// parseBSDTagLine attempts to parse a BSD tag format checksum line.
func parseBSDTagLine(line string) (filename, digest string, ok bool) {
	m := bsdTagRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[2], strings.ToLower(m[3]), true
}

// VerifyChecksums reads a checksum file, verifies each entry against the
// actual file contents, prints results according to opts, and returns whether
// all checks passed.
//
// R2.3: Checksum verification with CheckOptions.
func VerifyChecksums(checksumFile string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	f, err := openChecksumFile(checksumFile)
	if err != nil {
		return false, err
	}
	if f != os.Stdin {
		defer f.Close() // best-effort close
	}
	return verifyLines(f, cfg, opts, stdout, stderr)
}

// openChecksumFile opens the checksum file, or returns stdin for "-".
func openChecksumFile(name string) (*os.File, error) {
	if name == "-" {
		return os.Stdin, nil
	}
	return os.Open(name)
}

// verifyLines reads lines from r and verifies each checksum entry.
func verifyLines(r io.Reader, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	scanner := bufio.NewScanner(r)
	allOK := true
	malformed := 0
	for scanner.Scan() {
		line := scanner.Text()
		ok, isMalformed := verifyOneLine(line, cfg, opts, stdout, stderr)
		if !ok {
			allOK = false
		}
		if isMalformed {
			malformed++
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("reading checksum file: %w", err)
	}
	reportMalformed(malformed, opts, stderr)
	return allOK, nil
}

// verifyOneLine parses and verifies a single checksum line.
// Returns (passed, isMalformed).
func verifyOneLine(line string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, bool) {
	filename, expected, _, err := ParseChecksumLine(line, cfg)
	if err != nil {
		if opts.Warn && !opts.Status {
			fmt.Fprintf(stderr, "%s: no properly formatted checksum lines found\n", line)
		}
		return true, true
	}
	return verifyFile(filename, expected, cfg, opts, stdout, stderr), false
}

// verifyFile computes the digest of a file and compares it to the expected value.
func verifyFile(filename, expected string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) bool {
	actual, err := computeFileDigest(filename, cfg)
	if err != nil {
		if !opts.Status {
			fmt.Fprintf(stderr, "%s: FAILED open or read\n", filename)
		}
		return false
	}
	matched := actual == expected
	reportResult(filename, matched, opts, stdout)
	return matched
}

// computeFileDigest computes the hex digest of the named file.
func computeFileDigest(filename string, cfg HashConfig) (string, error) {
	r, _, err := openFileOrStdin(filename)
	if err != nil {
		return "", err
	}
	if r != os.Stdin {
		defer r.Close() // best-effort close
	}
	return ComputeDigest(r, cfg)
}

// reportResult prints the OK or FAILED result for a single file.
func reportResult(filename string, matched bool, opts CheckOptions, stdout io.Writer) {
	if opts.Status {
		return
	}
	if matched {
		if !opts.Quiet {
			fmt.Fprintf(stdout, "%s: OK\n", filename)
		}
		return
	}
	fmt.Fprintf(stdout, "%s: FAILED\n", filename)
}

// reportMalformed prints a warning summary for malformed lines.
func reportMalformed(count int, opts CheckOptions, stderr io.Writer) {
	if count > 0 && opts.Warn && !opts.Status {
		fmt.Fprintf(stderr, "WARNING: %d line is improperly formatted\n", count)
	}
}
