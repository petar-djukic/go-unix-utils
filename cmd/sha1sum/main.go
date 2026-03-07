// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements GNU sha1sum: compute and check SHA-1 message digests.
// Implements prd031-sha1sum R1-R4.
package main

import (
	"bufio"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// errPrefix is the program name used in error messages.
const errPrefix = "sha1sum"

// hashTag is the algorithm name used in BSD --tag output format.
const hashTag = "SHA1"

// options holds parsed command-line flag values.
type options struct {
	binary        bool // -b/--binary: use binary mode marker
	text          bool // -t/--text: use text mode marker (default)
	check         bool // -c/--check: verify checksums from file
	tag           bool // --tag: BSD-style output
	zero          bool // --zero: NUL line terminator instead of newline
	quiet         bool // --quiet: suppress OK lines in check mode
	status        bool // --status: no output in check mode, exit code only
	warn          bool // -w/--warn: warn about improperly formatted lines
	strict        bool // --strict: exit non-zero on improperly formatted lines
	ignoreMissing bool // --ignore-missing: skip missing files in check mode
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	if opts.check {
		os.Exit(runCheck(opts, files))
	}

	os.Exit(runHash(opts, files))
}

// runHash computes and prints SHA-1 digests for the given files. (prd031-sha1sum R1)
func runHash(opts options, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	for _, name := range files {
		hash, err := computeHash(name)
		if err != nil {
			// R1.4: print error to stderr and continue.
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", errPrefix, displayName(name), errorMessage(err))
			exitCode = 1
			continue
		}
		printHash(w, hash, name, opts)
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
		return 1
	}

	return exitCode
}

// computeHash reads a file (or stdin for "-") and returns its SHA-1 hex digest. (prd031-sha1sum R1.1, R1.2)
func computeHash(name string) (string, error) {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return "", err
		}
		defer f.Close()
		r = f
	}

	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// printHash writes a single hash line to the writer. (prd031-sha1sum R1.1, R1.3, R3.1, R3.2)
func printHash(w *bufio.Writer, hash, name string, opts options) {
	dn := displayName(name)
	terminator := "\n"
	if opts.zero {
		terminator = "\x00"
	}

	if opts.tag {
		// R1.3: BSD-style "SHA1 (FILENAME) = HASH"
		fmt.Fprintf(w, "%s (%s) = %s%s", hashTag, dn, hash, terminator)
	} else if opts.binary {
		// R3.1: binary mode "HASH *FILENAME"
		fmt.Fprintf(w, "%s *%s%s", hash, dn, terminator)
	} else {
		// R3.2: text mode (default) "HASH  FILENAME"
		fmt.Fprintf(w, "%s  %s%s", hash, dn, terminator)
	}
}

// displayName returns the display name for a file, using "-" for stdin.
func displayName(name string) string {
	if name == "-" {
		return "-"
	}
	return name
}

// runCheck verifies checksums from one or more checksum files. (prd031-sha1sum R2)
func runCheck(opts options, files []string) int {
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "%s: --check requires a file argument\n", errPrefix)
		return 1
	}

	w := bufio.NewWriter(os.Stdout)
	exitCode := 0

	for _, name := range files {
		if code := checkFile(w, name, opts); code != 0 {
			exitCode = code
		}
	}

	if err := w.Flush(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: write error: %v\n", errPrefix, err)
		return 1
	}

	return exitCode
}

// checkFile reads a checksum file and verifies each entry. (prd031-sha1sum R2.1, R2.2, R2.3)
func checkFile(w *bufio.Writer, name string, opts options) int {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %s\n", errPrefix, name, errorMessage(err))
			return 1
		}
		defer f.Close()
		r = f
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	failures := 0
	checked := 0
	malformed := 0

	for scanner.Scan() {
		line := scanner.Text()

		expectedHash, filename, ok := parseCheckLine(line)
		if !ok {
			malformed++
			if opts.warn {
				fmt.Fprintf(os.Stderr, "%s: %s: %d: improperly formatted %s checksum line\n",
					errPrefix, name, checked+malformed, hashTag)
			}
			continue
		}
		checked++

		actualHash, err := computeHash(filename)
		if err != nil {
			if opts.ignoreMissing && os.IsNotExist(unwrapErr(err)) {
				continue
			}
			if !opts.status {
				fmt.Fprintf(w, "%s: FAILED open or read\n", filename)
			}
			failures++
			continue
		}

		if strings.EqualFold(actualHash, expectedHash) {
			if !opts.quiet && !opts.status {
				fmt.Fprintf(w, "%s: OK\n", filename)
			}
		} else {
			if !opts.status {
				fmt.Fprintf(w, "%s: FAILED\n", filename)
			}
			failures++
		}
	}

	if scanner.Err() != nil {
		fmt.Fprintf(os.Stderr, "%s: %s: read error: %v\n", errPrefix, name, scanner.Err())
		return 1
	}

	if checked == 0 {
		fmt.Fprintf(os.Stderr, "%s: %s: no properly formatted %s checksum lines found\n", errPrefix, name, hashTag)
		return 1
	}

	if failures > 0 && !opts.status {
		fmt.Fprintf(os.Stderr, "%s: WARNING: %d computed checksum did NOT match\n", errPrefix, failures)
	}

	if opts.strict && malformed > 0 {
		return 1
	}

	if failures > 0 {
		return 1
	}

	return 0
}

// parseCheckLine parses a single line from a checksum file. Returns the expected
// hash, filename, and whether parsing succeeded. Supports both GNU format
// ("HASH  FILENAME" or "HASH *FILENAME") and BSD tag format ("SHA1 (FILENAME) = HASH").
// (prd031-sha1sum R2.1)
func parseCheckLine(line string) (hash, filename string, ok bool) {
	// Try BSD tag format: "SHA1 (FILENAME) = HASH"
	if strings.HasPrefix(line, hashTag+" (") {
		rest := line[len(hashTag)+2:]
		closeParen := strings.LastIndex(rest, ") = ")
		if closeParen < 0 {
			return "", "", false
		}
		filename = rest[:closeParen]
		hash = rest[closeParen+4:]
		if len(hash) != sha1.Size*2 {
			return "", "", false
		}
		return hash, filename, true
	}

	// GNU format: "HASH  FILENAME" or "HASH *FILENAME"
	if len(line) < sha1.Size*2+2 {
		return "", "", false
	}

	hash = line[:sha1.Size*2]
	sep := line[sha1.Size*2 : sha1.Size*2+2]

	if sep != "  " && sep != " *" {
		return "", "", false
	}

	filename = line[sha1.Size*2+2:]
	if filename == "" {
		return "", "", false
	}

	return hash, filename, true
}

// unwrapErr returns the innermost error from a *os.PathError chain.
func unwrapErr(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// errorMessage extracts the message from an error, unwrapping *os.PathError.
func errorMessage(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// parseArgs manually parses arguments, supporting GNU short and long forms.
func parseArgs(args []string) (options, []string) {
	var opts options
	var files []string

	i := 0
	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			files = append(files, args[i:]...)
			break
		}

		// Long options.
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--binary":
				opts.binary = true
			case "--text":
				opts.text = true
			case "--check":
				opts.check = true
			case "--tag":
				opts.tag = true
			case "--zero":
				opts.zero = true
			case "--quiet":
				opts.quiet = true
			case "--status":
				opts.status = true
			case "--warn":
				opts.warn = true
			case "--strict":
				opts.strict = true
			case "--ignore-missing":
				opts.ignoreMissing = true
			default:
				fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", errPrefix, arg)
				os.Exit(1)
			}
			i++
			continue
		}

		// Short options (e.g., -b, -t, -c, -w, -bw).
		if len(arg) > 1 && arg[0] == '-' {
			j := 1
			for j < len(arg) {
				switch arg[j] {
				case 'b':
					opts.binary = true
				case 't':
					opts.text = true
				case 'c':
					opts.check = true
				case 'w':
					opts.warn = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", errPrefix, arg[j])
					os.Exit(1)
				}
				j++
			}
			i++
			continue
		}

		// Positional argument.
		files = append(files, arg)
		i++
	}

	return opts, files
}
