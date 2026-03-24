// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd086-hashutil R2.3: VerifyChecksums for checksum file
// verification with CheckOptions support (Warn, Quiet, Status).
package hashutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// checkResult holds the counts from a checksum verification run.
type checkResult struct {
	failed    int
	malformed int
	allOK     bool
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
	res := checkResult{allOK: true}

	for scanner.Scan() {
		line := scanner.Text()
		ok, err := verifySingleLine(line, cfg, opts, stdout, stderr)
		if err != nil {
			res.malformed++
			continue
		}
		if !ok {
			res.allOK = false
			res.failed++
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("reading checksum file: %w", err)
	}
	progName := deriveProgramName(cfg)
	printSummaries(progName, res, opts, stderr)
	return res.allOK, nil
}

// verifySingleLine parses and verifies one checksum line.
func verifySingleLine(line string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	filename, expected, _, err := ParseChecksumLine(line, cfg)
	if err != nil {
		if opts.Warn {
			fmt.Fprintf(stderr, "%s: no properly formatted %s checksum lines found\n",
				line, cfg.Algorithm)
		}
		return true, err
	}
	return verifyFile(filename, expected, cfg, opts, stdout, stderr), nil
}

// verifyFile computes the digest of a file and compares it to expected.
func verifyFile(filename, expected string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) bool {
	actual, err := computeFileDigest(filename, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %s: %v\n", deriveProgramName(cfg), filename, err)
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

// deriveProgramName builds the program name from the algorithm (e.g., "MD5" -> "md5sum").
func deriveProgramName(cfg HashConfig) string {
	return strings.ToLower(cfg.Algorithm) + "sum"
}

// printSummaries prints end-of-check-mode summary warnings to stderr.
// R3.3 / R2.2: warns about malformed lines and failed checksums.
func printSummaries(progName string, res checkResult, opts CheckOptions, stderr io.Writer) {
	if !opts.Status {
		printMalformedSummary(progName, res.malformed, stderr)
	}
	printFailedSummary(progName, res.failed, opts, stderr)
}

// printMalformedSummary prints the malformed line count warning.
func printMalformedSummary(progName string, count int, stderr io.Writer) {
	if count == 0 {
		return
	}
	verb := "is"
	noun := "line"
	if count > 1 {
		verb = "are"
		noun = "lines"
	}
	fmt.Fprintf(stderr, "%s: WARNING: %d %s %s improperly formatted\n",
		progName, count, noun, verb)
}

// printFailedSummary prints the failed checksum count warning.
func printFailedSummary(progName string, count int, opts CheckOptions, stderr io.Writer) {
	if count == 0 {
		return
	}
	if opts.Status {
		return
	}
	noun := "checksum"
	if count > 1 {
		noun = "checksums"
	}
	fmt.Fprintf(stderr, "%s: WARNING: %d computed %s did NOT match\n",
		progName, count, noun)
}
