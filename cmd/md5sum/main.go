// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd030-md5sum R1.1, R1.2, R1.3, R1.4: MD5 digest computation
// with GNU and BSD tag output formats.
package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]
	binary, tag, files := parseArgs(args)

	if len(files) == 0 {
		files = []string{"-"}
	}

	exitCode := 0
	for _, file := range files {
		if err := processFile(file, binary, tag); err != nil {
			printError(file, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// parseArgs extracts flags and file arguments from the command line.
// R1.1: --binary/-b, --text/-t, R1.3: --tag.
func parseArgs(args []string) (binary bool, tag bool, files []string) {
	for _, arg := range args {
		switch arg {
		case "-b", "--binary":
			binary = true
		case "-t", "--text":
			binary = false
		case "--tag":
			tag = true
		case "--":
			continue
		default:
			files = append(files, arg)
		}
	}
	return binary, tag, files
}

// processFile computes the MD5 digest for a single file or stdin and
// prints the result. R1.1, R1.2, R1.3, R1.4.
func processFile(name string, binary, tag bool) error {
	digest, err := computeDigest(name)
	if err != nil {
		return err
	}
	printDigest(name, digest, binary, tag)
	return nil
}

// computeDigest reads the file (or stdin for "-") and returns its MD5 hex digest.
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
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// printError formats an error message matching GNU md5sum style.
// Extracts the underlying syscall error to avoid duplicating the filename.
func printError(name string, err error) {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "md5sum: %s: %s\n", name, pathErr.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "md5sum: %s: %v\n", name, err)
}

// printDigest formats and prints the digest line.
// R1.1: GNU format "HASH  FILENAME" (text) or "HASH *FILENAME" (binary).
// R1.2: stdin shown as "-".
// R1.3: BSD tag format "MD5 (FILENAME) = HASH".
func printDigest(name, digest string, binary, tag bool) {
	displayName := name
	if tag {
		fmt.Printf("MD5 (%s) = %s\n", displayName, digest)
		return
	}
	if binary {
		fmt.Printf("%s *%s\n", digest, displayName)
		return
	}
	fmt.Printf("%s  %s\n", digest, displayName)
}
