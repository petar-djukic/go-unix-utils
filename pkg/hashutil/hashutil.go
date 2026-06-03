// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

package hashutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
)

var (
	bsdTagRe = regexp.MustCompile(`^(\S+) \((.+)\) = ([0-9a-fA-F]+)$`)
	gnuRe    = regexp.MustCompile(`^([0-9a-fA-F]+) ([ *])(.+)$`)
)

// ParseChecksumLine parses a single GNU or BSD tag format checksum line. (R2.1)
func ParseChecksumLine(line string, cfg HashConfig) (filename, expectedDigest string, binary bool, err error) {
	if m := bsdTagRe.FindStringSubmatch(line); m != nil {
		return m[2], m[3], false, nil
	}
	if m := gnuRe.FindStringSubmatch(line); m != nil {
		return m[3], m[1], m[2] == "*", nil
	}
	return "", "", false, fmt.Errorf("malformed checksum line")
}

// VerifyChecksums reads checksumFile, verifies each entry, and prints
// results according to opts. Returns whether all checks passed. (R2.3)
func VerifyChecksums(checksumFile string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	r, err := openInput(checksumFile)
	if err != nil {
		return false, fmt.Errorf("opening checksum file: %w", err)
	}
	defer r.Close()
	return verifyLines(r, cfg, opts, stdout, stderr)
}

func verifyLines(r io.Reader, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) (bool, error) {
	scanner := bufio.NewScanner(r)
	allOK := true
	for scanner.Scan() {
		if !verifyOneLine(scanner.Text(), cfg, opts, stdout, stderr) {
			allOK = false
		}
	}
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("reading checksum input: %w", err)
	}
	return allOK, nil
}

func verifyOneLine(line string, cfg HashConfig, opts CheckOptions, stdout, stderr io.Writer) bool {
	filename, expected, _, err := ParseChecksumLine(line, cfg)
	if err != nil {
		if opts.Warn && !opts.Status {
			fmt.Fprintf(stderr, "WARNING: improperly formatted checksum line\n")
		}
		return true
	}
	actual, err := computeFileDigest(filename, cfg)
	if err != nil {
		if !opts.Status {
			fmt.Fprintf(stdout, "%s: FAILED open or read\n", filename)
		}
		return false
	}
	if actual != expected {
		if !opts.Status {
			fmt.Fprintf(stdout, "%s: FAILED\n", filename)
		}
		return false
	}
	if !opts.Status && !opts.Quiet {
		fmt.Fprintf(stdout, "%s: OK\n", filename)
	}
	return true
}

func computeFileDigest(filename string, cfg HashConfig) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return ComputeDigest(f, cfg)
}

// DigestFiles computes and prints digests for each file (or stdin when files
// is empty or contains "-"), returning the exit code. (R3.1, R3.2)
func DigestFiles(files []string, cfg HashConfig, binary, tag bool, stdout, stderr io.Writer) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, file := range files {
		if err := digestAndPrint(file, cfg, binary, tag, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: %s\n", file, err)
			exitCode = 1
		}
	}
	return exitCode
}

func digestAndPrint(file string, cfg HashConfig, binary, tag bool, stdout io.Writer) error {
	r, err := openInput(file)
	if err != nil {
		return err
	}
	defer r.Close()
	digest, err := ComputeDigest(r, cfg)
	if err != nil {
		return err
	}
	if tag {
		fmt.Fprintln(stdout, FormatBSDTag(cfg.Algorithm, file, digest))
	} else {
		fmt.Fprintln(stdout, FormatGNU(digest, file, binary))
	}
	return nil
}

func openInput(file string) (io.ReadCloser, error) {
	if file == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(file)
}
