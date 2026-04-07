// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package hashutil provides shared hash output formatting and checksum
// verification logic used by cmd/ hash utilities (md5sum, sha1sum, etc.).
//
// Implements: srd086-hashutil R1 (hash output formatting), R2 (checksum
// verification), R3 (file processing).
package hashutil

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"regexp"
	"strings"
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

// bsdTagRe matches BSD tag format: "ALGORITHM (FILENAME) = HASH".
var bsdTagRe = regexp.MustCompile(`^(\S+) \((.+)\) = ([0-9a-fA-F]+)$`)

// gnuRe matches GNU format: "HASH  FILENAME" (text) or "HASH *FILENAME" (binary).
var gnuRe = regexp.MustCompile(`^([0-9a-fA-F]+) ([ *])(.+)$`)

// errMalformedLine is returned when a checksum line cannot be parsed.
var errMalformedLine = errors.New("malformed checksum line")

// FormatGNU returns a GNU-format checksum line: "HASH  FILENAME" for text
// mode or "HASH *FILENAME" for binary mode.
// R1.2: two-space separator for text mode, space-asterisk for binary mode.
func FormatGNU(digest, filename string, binary bool) string {
	if binary {
		return digest + " *" + filename
	}
	return digest + "  " + filename
}

// FormatBSDTag returns a BSD tag-format checksum line:
// "ALGORITHM (FILENAME) = HASH".
// R1.3: uses the algorithm name, parenthesized filename, and digest.
func FormatBSDTag(algorithm, filename, digest string) string {
	return algorithm + " (" + filename + ") = " + digest
}

// ComputeDigest reads all bytes from r, computes the hash using cfg, and
// returns the lowercase hex-encoded digest string.
// R1.4: delegates to cfg.NewHash for the hash instance.
func ComputeDigest(r io.Reader, cfg HashConfig) (string, error) {
	h := cfg.NewHash()
	if _, err := io.Copy(h, r); err != nil {
		return "", fmt.Errorf("computing digest: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ParseChecksumLine parses a single checksum line in GNU format
// ("HASH [ *]FILENAME") or BSD tag format ("ALGORITHM (FILENAME) = HASH").
// Returns the filename, expected digest, binary flag, and an error for
// malformed lines or digest length mismatch.
// R2.1: supports both GNU and BSD tag format detection.
func ParseChecksumLine(line string, cfg HashConfig) (filename, expectedDigest string, binary bool, err error) {
	line = strings.TrimRight(line, "\r\n")
	if m := bsdTagRe.FindStringSubmatch(line); m != nil {
		return parsedWithLenCheck(m[2], m[3], false, cfg)
	}
	if m := gnuRe.FindStringSubmatch(line); m != nil {
		return parsedWithLenCheck(m[3], m[1], m[2] == "*", cfg)
	}
	return "", "", false, errMalformedLine
}

// parsedWithLenCheck validates digest length and returns parsed fields.
func parsedWithLenCheck(name, digest string, bin bool, cfg HashConfig) (string, string, bool, error) {
	d := strings.ToLower(digest)
	if cfg.DigestLen > 0 && len(d) != cfg.DigestLen {
		return "", "", false, errMalformedLine
	}
	return name, d, bin, nil
}

// VerifyChecksums reads a checksum file, verifies each entry against the
// actual file contents, prints results according to opts, and returns
// whether all checks passed.
// R2.3: respects CheckOptions for output control.
func VerifyChecksums(checksumFile string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	f, err := os.Open(checksumFile)
	if err != nil {
		return false, fmt.Errorf("opening checksum file: %w", err)
	}
	defer f.Close() // best-effort close on read-only file

	allOK := true
	failures := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if isSkippableLine(line) {
			continue
		}
		ok := verifyOneLine(line, cfg, opts, stdout, stderr)
		if !ok {
			allOK = false
			failures++
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("reading checksum file: %w", err)
	}
	if failures > 0 && !opts.Status {
		fmt.Fprintf(stderr, "WARNING: %d computed checksum did NOT match\n", failures)
	}
	return allOK, nil
}

// isSkippableLine returns true for blank lines and comment lines.
// R2.3: skip blank lines and lines starting with #.
func isSkippableLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.HasPrefix(trimmed, "#")
}

// verifyOneLine parses, computes, and compares one checksum line.
// Returns true if the digest matches.
func verifyOneLine(line string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) bool {
	name, expected, _, err := ParseChecksumLine(line, cfg)
	if err != nil {
		if opts.Warn {
			fmt.Fprintf(stderr, "WARNING: %d line is improperly formatted\n", 1)
		}
		return false
	}
	return verifyFile(name, expected, cfg, opts, stdout, stderr)
}

// verifyFile opens a file, computes its digest, and reports the result.
func verifyFile(name, expected string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) bool {
	actual, err := computeFileDigest(name, cfg)
	if err != nil {
		if !opts.Status {
			fmt.Fprintf(stderr, "%s: FAILED open or read\n", name)
		}
		return false
	}
	matched := actual == expected
	if !opts.Status {
		reportMatch(name, matched, opts, stdout)
	}
	return matched
}

// computeFileDigest opens a file and computes its digest.
func computeFileDigest(name string, cfg HashConfig) (string, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer f.Close() // best-effort close on read-only file
	return ComputeDigest(f, cfg)
}

// reportMatch writes the OK or FAILED line for a verified file.
func reportMatch(name string, matched bool, opts CheckOptions, stdout io.Writer) {
	if matched {
		if !opts.Quiet {
			fmt.Fprintf(stdout, "%s: OK\n", name)
		}
		return
	}
	fmt.Fprintf(stdout, "%s: FAILED\n", name)
}

// DigestFiles computes and prints digests for each file in files (or stdin
// when files is empty or contains "-"). Returns exit code 0 for success,
// 1 if any file failed.
// R3.1: formats output as GNU or BSD tag depending on the tag flag.
// R3.2: continues processing remaining files on error.
func DigestFiles(files []string, cfg HashConfig, binary, tag bool, stdout, stderr io.Writer) int {
	panic("hashutil.DigestFiles: not yet implemented")
}
