// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd033-sha512sum R1.1, R1.2, R1.3, R1.4: SHA-512 digest computation
// with GNU and BSD tag output formats.
// Implements prd033-sha512sum R2.1, R2.2, R2.3: checksum verification
// with --check, --warn, --quiet, --status.
// Implements prd033-sha512sum R3.1, R3.2: binary/text mode flags and
// tag format interaction.
// Implements prd033-sha512sum R4.1, R4.2, R4.3: exit codes and SIGPIPE handling.
package main

import (
	"bufio"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// gnuPattern matches GNU-format checksum lines: "HASH  FILENAME" or "HASH *FILENAME".
var gnuPattern = regexp.MustCompile(`^([0-9a-fA-F]{128}) [ *](.+)$`)

// bsdPattern matches BSD tag format: "SHA512 (FILENAME) = HASH".
var bsdPattern = regexp.MustCompile(`^SHA512 \((.+)\) = ([0-9a-fA-F]{128})$`)

// options holds parsed command-line flags.
type options struct {
	binary       bool
	textExplicit bool // true when -t/--text was explicitly passed
	tag          bool
	check        bool
	warn         bool
	quiet        bool
	status       bool
	files        []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts := parseArgs(os.Args[1:])

	if len(opts.files) == 0 {
		opts.files = []string{"-"}
	}

	// R3.2: --tag does not support explicit --text mode.
	if opts.tag && opts.textExplicit {
		fmt.Fprintf(os.Stderr, "sha512sum: --tag does not support --text mode\n")
		os.Exit(1)
	}

	if opts.check {
		os.Exit(runCheck(opts))
	}
	os.Exit(runDigest(opts))
}

// parseArgs extracts flags and file arguments from the command line.
// R1.1: --binary/-b, --text/-t. R1.3: --tag.
// R2.1: --check/-c. R2.3: --warn/-w, --quiet, --status.
func parseArgs(args []string) options {
	var opts options
	for _, arg := range args {
		switch arg {
		case "-b", "--binary":
			opts.binary = true
		case "-t", "--text":
			opts.binary = false
			opts.textExplicit = true
		case "--tag":
			opts.tag = true
		case "-c", "--check":
			opts.check = true
		case "-w", "--warn":
			opts.warn = true
		case "--quiet":
			opts.quiet = true
		case "--status":
			opts.status = true
		case "--":
			continue
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return opts
}

// runDigest computes and prints digests for all files. Returns exit code.
// R1.4: exit 1 on error, continue processing remaining files.
func runDigest(opts options) int {
	exitCode := 0
	for _, file := range opts.files {
		if err := processFile(file, opts.binary, opts.tag); err != nil {
			printError(file, err)
			exitCode = 1
		}
	}
	return exitCode
}

// processFile computes the SHA-512 digest for a single file or stdin and
// prints the result. R1.1, R1.2, R1.3, R1.4.
func processFile(name string, binary, tag bool) error {
	digest, err := computeDigest(name)
	if err != nil {
		return err
	}
	printDigest(name, digest, binary, tag)
	return nil
}

// computeDigest reads the file (or stdin for "-") and returns its SHA-512 hex digest.
// R1.2: reads stdin when name is "-".
func computeDigest(name string) (string, error) {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return "", err
		}
		defer f.Close() // best-effort close on read-only file
		r = f
	}
	h := sha512.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// printError formats an error message matching GNU sha512sum style.
// Extracts the underlying syscall error to avoid duplicating the filename.
func printError(name string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "sha512sum: %s: %s\n", name, pathErr.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "sha512sum: %s: %v\n", name, err)
}

// printDigest formats and prints the digest line.
// R1.1: GNU format "HASH  FILENAME" (text) or "HASH *FILENAME" (binary).
// R1.2: stdin shown as "-".
// R1.3: BSD tag format "SHA512 (FILENAME) = HASH".
// R3.1: -b uses "HASH *FILENAME", -t (default) uses "HASH  FILENAME".
func printDigest(name, digest string, binary, tag bool) {
	if tag {
		fmt.Printf("SHA512 (%s) = %s\n", name, digest)
		return
	}
	if binary {
		fmt.Printf("%s *%s\n", digest, name)
		return
	}
	fmt.Printf("%s  %s\n", digest, name)
}

// runCheck reads checksum files and verifies digests. Returns exit code.
// R2.1, R2.2, R2.3.
func runCheck(opts options) int {
	exitCode := 0
	for _, file := range opts.files {
		if rc := checkFile(file, opts); rc != 0 {
			exitCode = rc
		}
	}
	return exitCode
}

// checkFile reads a single checksum file and verifies each entry.
// Returns 0 if all entries pass, 1 if any fail or the file can't be read.
func checkFile(name string, opts options) int {
	r, closer, err := openInput(name)
	if err != nil {
		printError(name, err)
		return 1
	}
	if closer != nil {
		defer closer.Close() // best-effort close on read-only file
	}
	return verifyEntries(r, name, opts)
}

// openInput opens a file or returns stdin for "-".
func openInput(name string) (io.Reader, io.Closer, error) {
	if name == "-" {
		return os.Stdin, nil, nil
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, err
	}
	return f, f, nil
}

// verifyResult holds counts from checksum verification.
type verifyResult struct {
	failedCount    int
	malformedCount int
}

// verifyEntries scans lines from a checksum file and verifies each.
// Returns 0 if all pass, 1 if any fail.
func verifyEntries(r io.Reader, name string, opts options) int {
	result := scanAndVerify(r, name, opts)
	printSummaryWarnings(result, opts)
	if result.failedCount > 0 {
		return 1
	}
	return 0
}

// scanAndVerify processes each line in a checksum file.
func scanAndVerify(r io.Reader, name string, opts options) verifyResult {
	scanner := bufio.NewScanner(r)
	lineNum := 0
	var result verifyResult

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		hash, filename, ok := parseLine(line)
		if !ok {
			result.malformedCount++
			if opts.warn {
				// R2.3: warn on malformed lines
				fmt.Fprintf(os.Stderr,
					"sha512sum: %s: %d: improperly formatted SHA512 checksum line\n",
					name, lineNum)
			}
			continue
		}
		if !verifyOneEntry(hash, filename, opts) {
			result.failedCount++
		}
	}
	return result
}

// printSummaryWarnings prints GNU-style summary warnings to stderr.
func printSummaryWarnings(result verifyResult, opts options) {
	if !opts.status && result.malformedCount > 0 {
		noun := "lines are"
		if result.malformedCount == 1 {
			noun = "line is"
		}
		fmt.Fprintf(os.Stderr,
			"sha512sum: WARNING: %d %s improperly formatted\n",
			result.malformedCount, noun)
	}
	if !opts.status && result.failedCount > 0 {
		noun := "computed checksums did"
		if result.failedCount == 1 {
			noun = "computed checksum did"
		}
		fmt.Fprintf(os.Stderr,
			"sha512sum: WARNING: %d %s NOT match\n",
			result.failedCount, noun)
	}
}

// parseLine tries GNU then BSD format. Returns hash, filename, ok.
// R2.1: Supports "HASH  FILENAME", "HASH *FILENAME", and "SHA512 (FILENAME) = HASH".
func parseLine(line string) (hash, filename string, ok bool) {
	if m := gnuPattern.FindStringSubmatch(line); m != nil {
		return strings.ToLower(m[1]), m[2], true
	}
	if m := bsdPattern.FindStringSubmatch(line); m != nil {
		return strings.ToLower(m[2]), m[1], true
	}
	return "", "", false
}

// verifyOneEntry computes the digest of filename and compares to expected.
// R2.2: Prints "FILENAME: OK" or "FILENAME: FAILED".
// R2.3: --quiet suppresses OK; --status suppresses all output.
func verifyOneEntry(expectedHash, filename string, opts options) bool {
	actual, err := computeDigest(filename)
	if err != nil {
		if !opts.status {
			fmt.Printf("%s: FAILED open or read\n", filename)
		}
		printError(filename, err)
		return false
	}
	match := actual == expectedHash
	if opts.status {
		return match
	}
	if match && !opts.quiet {
		fmt.Printf("%s: OK\n", filename)
	}
	if !match {
		fmt.Printf("%s: FAILED\n", filename)
	}
	return match
}
