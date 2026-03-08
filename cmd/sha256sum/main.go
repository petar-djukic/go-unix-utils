// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements the sha256sum utility for computing and verifying SHA-256 digests.
//
// Implements prd032-sha256sum: digest computation (R1), checksum verification (R2),
// binary/text mode flags (R3), exit codes and SIGPIPE (R4).
package main

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// hashLength is the length of a SHA-256 hex digest string.
const hashLength = 64

// flags holds the parsed command-line options.
type flags struct {
	binary bool // -b/--binary: use binary mode indicator
	text   bool // -t/--text: use text mode indicator (default)
	check  bool // -c/--check: verify checksums from file
	tag    bool // --tag: BSD-style output format
	quiet  bool // --quiet: suppress OK lines in check mode
	status bool // --status: suppress all output in check mode
	warn   bool // -w/--warn: warn about malformed checksum lines
	strict bool // --strict: exit non-zero for malformed lines (accepted but PRD says non-goal)
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, files := parseArgs(os.Args[1:])

	if f.check {
		os.Exit(runCheck(f, files))
	}
	os.Exit(runCompute(f, files))
}

// parseArgs parses command-line arguments into flags and file names.
func parseArgs(args []string) (flags, []string) {
	var f flags
	var files []string
	endOfFlags := false

	for _, arg := range args {

		if endOfFlags {
			files = append(files, arg)
			continue
		}

		if arg == "--" {
			endOfFlags = true
			continue
		}

		if arg == "-" {
			files = append(files, arg)
			continue
		}

		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--binary":
				f.binary = true
			case "--text":
				f.text = true
			case "--check":
				f.check = true
			case "--tag":
				f.tag = true
			case "--quiet":
				f.quiet = true
			case "--status":
				f.status = true
			case "--warn":
				f.warn = true
			case "--strict":
				f.strict = true
			default:
				fmt.Fprintf(os.Stderr, "sha256sum: unrecognized option '%s'\n", arg)
				os.Exit(1)
			}
			continue
		}

		if arg[0] == '-' {
			for _, ch := range arg[1:] {
				switch ch {
				case 'b':
					f.binary = true
				case 't':
					f.text = true
				case 'c':
					f.check = true
				case 'w':
					f.warn = true
				default:
					fmt.Fprintf(os.Stderr, "sha256sum: invalid option -- '%c'\n", ch)
					os.Exit(1)
				}
			}
			continue
		}

		files = append(files, arg)
	}

	return f, files
}

// runCompute computes SHA-256 digests for files or stdin. R1.
func runCompute(f flags, files []string) int {
	exitCode := 0
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	if len(files) == 0 {
		files = []string{"-"}
	}

	for _, name := range files {
		hash, err := computeHash(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sha256sum: %s: %s\n", name, errorMessage(err))
			exitCode = 1
			continue
		}
		printHash(w, hash, name, f)
	}

	return exitCode
}

// computeHash computes the SHA-256 hex digest of a file or stdin ("-").
func computeHash(name string) (string, error) {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		file, err := os.Open(name)
		if err != nil {
			return "", err
		}
		defer file.Close()
		r = file
	}

	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// printHash writes a single hash line in the appropriate format.
func printHash(w *bufio.Writer, hash, name string, f flags) {
	// R1.3: --tag uses BSD format.
	if f.tag {
		fmt.Fprintf(w, "SHA256 (%s) = %s\n", name, hash)
		return
	}

	// R3.1: -b uses " *" separator; R3.2: default text uses "  " separator.
	if f.binary {
		fmt.Fprintf(w, "%s *%s\n", hash, name)
	} else {
		fmt.Fprintf(w, "%s  %s\n", hash, name)
	}
}

// errorMessage extracts a user-friendly error message from an os error.
func errorMessage(err error) string {
	if os.IsNotExist(err) {
		return "No such file or directory"
	}
	if os.IsPermission(err) {
		return "Permission denied"
	}
	return err.Error()
}

// runCheck reads a checksum file and verifies each entry. R2.
func runCheck(f flags, files []string) int {
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "sha256sum: --check requires a file argument\n")
		return 1
	}

	exitCode := 0
	for _, name := range files {
		if checkFile(f, name) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// checkFile processes a single checksum file. Returns 0 on success, 1 on any failure.
func checkFile(f flags, name string) int {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		file, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sha256sum: %s: %s\n", name, errorMessage(err))
			return 1
		}
		defer file.Close()
		r = file
	}

	scanner := bufio.NewScanner(r)
	failures := 0
	malformed := 0
	checked := 0

	for lineNum := 1; scanner.Scan(); lineNum++ {
		line := scanner.Text()
		hash, filename, ok := parseChecksumLine(line)
		if !ok {
			malformed++
			if f.warn {
				fmt.Fprintf(os.Stderr, "sha256sum: %s: %d: improperly formatted SHA256 checksum line\n", name, lineNum)
			}
			continue
		}

		actual, err := computeHash(filename)
		checked++
		if err != nil {
			if !f.status {
				fmt.Fprintf(os.Stdout, "%s: FAILED open or read\n", filename)
			}
			failures++
			continue
		}

		if strings.EqualFold(actual, hash) {
			if !f.status && !f.quiet {
				fmt.Fprintf(os.Stdout, "%s: OK\n", filename)
			}
		} else {
			if !f.status {
				fmt.Fprintf(os.Stdout, "%s: FAILED\n", filename)
			}
			failures++
		}
	}

	if failures > 0 {
		if !f.status {
			fmt.Fprintf(os.Stderr, "sha256sum: WARNING: %d computed checksum did NOT match\n", failures)
		}
		return 1
	}

	if checked == 0 && malformed > 0 {
		fmt.Fprintf(os.Stderr, "sha256sum: %s: no properly formatted SHA256 checksum lines found\n", name)
		return 1
	}

	return 0
}

// parseChecksumLine parses a line in GNU format ("HASH  FILENAME" or "HASH *FILENAME")
// or BSD tag format ("SHA256 (FILENAME) = HASH"). R2.1.
func parseChecksumLine(line string) (hash, filename string, ok bool) {
	// Try BSD tag format: "SHA256 (FILENAME) = HASH"
	if strings.HasPrefix(line, "SHA256 (") {
		rest := line[8:] // after "SHA256 ("
		before, after, found := strings.Cut(rest, ") = ")
		if !found {
			return "", "", false
		}
		filename = before
		hash = after
		if len(hash) != hashLength {
			return "", "", false
		}
		return hash, filename, true
	}

	// GNU format: "HASH  FILENAME" or "HASH *FILENAME"
	// hashLength hex + 2-char separator + at least 1 char filename
	if len(line) < hashLength+2+1 {
		return "", "", false
	}

	hash = line[:hashLength]
	// Validate hex characters.
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", "", false
		}
	}

	sep := line[hashLength : hashLength+2]
	if sep != "  " && sep != " *" {
		return "", "", false
	}

	filename = line[hashLength+2:]
	if filename == "" {
		return "", "", false
	}

	return hash, filename, true
}
