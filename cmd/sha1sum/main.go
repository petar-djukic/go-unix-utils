// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd031-sha1sum R1.1, R1.2, R1.3, R1.4: SHA-1 digest computation
// with GNU and BSD tag output formats.
package main

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// options holds parsed command-line flags.
type options struct {
	binary bool
	tag    bool
	files  []string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts := parseArgs(os.Args[1:])

	if len(opts.files) == 0 {
		opts.files = []string{"-"}
	}

	os.Exit(runDigest(opts))
}

// parseArgs extracts flags and file arguments from the command line.
// R1.1: --binary/-b, --text/-t. R1.3: --tag.
func parseArgs(args []string) options {
	var opts options
	for _, arg := range args {
		switch arg {
		case "-b", "--binary":
			opts.binary = true
		case "-t", "--text":
			opts.binary = false
		case "--tag":
			opts.tag = true
		case "--":
			continue
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return opts
}

// runDigest computes and prints digests for all files. Returns exit code.
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

// processFile computes the SHA-1 digest for a single file or stdin and
// prints the result. R1.1, R1.2, R1.3, R1.4.
func processFile(name string, binary, tag bool) error {
	digest, err := computeDigest(name)
	if err != nil {
		return err
	}
	printDigest(name, digest, binary, tag)
	return nil
}

// computeDigest reads the file (or stdin for "-") and returns its SHA-1 hex digest.
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
	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// printError formats an error message matching GNU sha1sum style.
// Extracts the underlying syscall error to avoid duplicating the filename.
func printError(name string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "sha1sum: %s: %s\n", name, pathErr.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "sha1sum: %s: %v\n", name, err)
}

// printDigest formats and prints the digest line.
// R1.1: GNU format "HASH  FILENAME" (text) or "HASH *FILENAME" (binary).
// R1.2: stdin shown as "-".
// R1.3: BSD tag format "SHA1 (FILENAME) = HASH".
func printDigest(name, digest string, binary, tag bool) {
	if tag {
		fmt.Printf("SHA1 (%s) = %s\n", name, digest)
		return
	}
	if binary {
		fmt.Printf("%s *%s\n", digest, name)
		return
	}
	fmt.Printf("%s  %s\n", digest, name)
}
