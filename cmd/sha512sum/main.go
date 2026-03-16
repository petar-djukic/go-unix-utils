// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd033-sha512sum R1.1-R1.4, R2.1-R2.3: core SHA-512 digest
// computation, standard GNU output format, and --check verification mode
// with OK/FAILED status output and summary warnings. Computes SHA-512
// digests for files or stdin, printing one line per input as 128 lowercase
// hex characters followed by two spaces and the filename. In check mode,
// reads a checksum file and verifies each listed file. Installs SIGPIPE
// handler for clean exit on broken pipe.
package main

import (
	"bufio"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU sha512sum format.
const progName = "sha512sum"

// sha512HexLen is the length of a SHA-512 digest in hexadecimal characters.
const sha512HexLen = 128

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	if opts.helpRequested {
		printHelp()
		os.Exit(0)
	}
	if opts.versionRequested {
		printVersion()
		os.Exit(0)
	}

	var exitCode int
	if opts.check {
		exitCode = runCheck(files)
	} else {
		exitCode = run(files)
	}
	os.Exit(exitCode)
}

// options holds parsed command-line flag state.
type options struct {
	helpRequested    bool
	versionRequested bool
	check            bool
}

// run processes files and returns the exit code.
// R1.1: prints "HASH  FILENAME" for each file.
// R1.2: reads stdin when no files given or "-" specified.
// R1.3: processes multiple files in order, continues on error.
func run(files []string) int {
	exitCode := 0

	if len(files) == 0 {
		// R1.2: no file arguments — read from stdin.
		if err := hashReader(os.Stdin, "-"); err != nil {
			if isEPIPE(err) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "%s: -: %v\n", progName, err)
			return 1
		}
		return 0
	}

	// R1.3: process multiple file arguments sequentially.
	for _, name := range files {
		if name == "-" {
			// R1.2: "-" means read from stdin.
			if err := hashReader(os.Stdin, "-"); err != nil {
				if isEPIPE(err) {
					os.Exit(0)
				}
				fmt.Fprintf(os.Stderr, "%s: -: %v\n", progName, err)
				exitCode = 1
			}
			continue
		}

		f, err := os.Open(name)
		if err != nil {
			// R1.3: print error to stderr, continue processing remaining files.
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		if err := hashReader(f, name); err != nil {
			f.Close() // best-effort close
			if isEPIPE(err) {
				os.Exit(0)
			}
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
			exitCode = 1
			continue
		}
		f.Close() // best-effort close
	}

	return exitCode
}

// hashReader computes the SHA-512 digest of r and writes one output line.
// R1.1: format is "HASH  FILENAME" (128 lowercase hex characters, two spaces, filename).
func hashReader(r io.Reader, name string) error {
	h := sha512.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}

	digest := fmt.Sprintf("%x", h.Sum(nil))
	_, err := fmt.Fprintf(os.Stdout, "%s  %s\n", digest, name)
	return err
}

// checkResult tracks the outcome of check mode verification.
type checkResult struct {
	mismatched int
	unreadable int
}

// runCheck reads one or more checksum files and verifies each listed file.
// R2.1: parses GNU format lines. R2.2: prints OK/FAILED.
// R2.3: summary warning on stderr. Exit 0 on all-pass, 1 on any failure.
func runCheck(files []string) int {
	if len(files) == 0 {
		// --check with no file argument reads from stdin.
		files = []string{"-"}
	}

	var total checkResult
	exitCode := 0

	for _, name := range files {
		result, err := checkFile(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		total.mismatched += result.mismatched
		total.unreadable += result.unreadable
	}

	// R2.3: GNU prints separate summary warnings for unreadable files and mismatches.
	if total.unreadable > 0 {
		fmt.Fprintf(os.Stderr, "%s: WARNING: %d listed file could not be read\n", progName, total.unreadable)
		exitCode = 1
	}
	if total.mismatched > 0 {
		fmt.Fprintf(os.Stderr, "%s: WARNING: %d computed checksum did NOT match\n", progName, total.mismatched)
		exitCode = 1
	}

	return exitCode
}

// checkFile opens a checksum file (or stdin for "-"), parses each line, verifies
// the digest, and prints OK/FAILED. Returns counts of mismatches and unreadable files.
func checkFile(name string) (checkResult, error) {
	var r io.Reader
	if name == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(name)
		if err != nil {
			return checkResult{}, err
		}
		defer f.Close()
		r = f
	}

	var result checkResult
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		hash, filename, ok := parseCheckLine(line)
		if !ok {
			continue
		}

		computed, err := computeFileHash(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, filename, unwrapPathError(err))
			// GNU prints "FILENAME: FAILED open or read" for unreadable files.
			fmt.Fprintf(os.Stdout, "%s: FAILED open or read\n", filename)
			result.unreadable++
			continue
		}

		if computed == hash {
			fmt.Fprintf(os.Stdout, "%s: OK\n", filename)
		} else {
			fmt.Fprintf(os.Stdout, "%s: FAILED\n", filename)
			result.mismatched++
		}
	}

	if err := scanner.Err(); err != nil {
		return result, err
	}

	return result, nil
}

// parseCheckLine parses a single line from a checksum file.
// Supports GNU format "HASH  FILENAME" or "HASH *FILENAME", and BSD tag format
// "SHA512 (FILENAME) = HASH".
// Returns the lowercase hex hash, the filename, and whether the line was valid.
func parseCheckLine(line string) (hash, filename string, ok bool) {
	// R2.1: Try BSD tag format first: "SHA512 (FILENAME) = HASH"
	if strings.HasPrefix(line, "SHA512 (") {
		return parseBSDTagLine(line)
	}

	// GNU text mode: "hash  filename"
	// GNU binary mode: "hash *filename"
	if len(line) < sha512HexLen+2 {
		return "", "", false
	}

	hash = line[:sha512HexLen]
	if !isValidHex(hash) {
		return "", "", false
	}

	sep := line[sha512HexLen : sha512HexLen+2]
	if sep != "  " && sep != " *" {
		return "", "", false
	}

	filename = line[sha512HexLen+2:]
	if filename == "" {
		return "", "", false
	}

	return strings.ToLower(hash), filename, true
}

// parseBSDTagLine parses a BSD tag format line: "SHA512 (FILENAME) = HASH".
func parseBSDTagLine(line string) (hash, filename string, ok bool) {
	// "SHA512 (" is 8 characters.
	closeIdx := strings.LastIndex(line, ") = ")
	if closeIdx < 8 {
		return "", "", false
	}

	filename = line[8:closeIdx]
	if filename == "" {
		return "", "", false
	}

	hash = line[closeIdx+4:]
	if len(hash) != sha512HexLen || !isValidHex(hash) {
		return "", "", false
	}

	return strings.ToLower(hash), filename, true
}

// isValidHex returns true if s contains only hexadecimal characters.
func isValidHex(s string) bool {
	for _, ch := range s {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return true
}

// computeFileHash computes the SHA-512 hex digest of the named file.
func computeFileHash(name string) (string, error) {
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

	h := sha512.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// parseArgs separates flags from file arguments. Supports -c/--check,
// --help, --version, and -- to end flag parsing. Single-char flags can be grouped.
func parseArgs(args []string) (opts options, files []string) {
	flagsDone := false

	for _, arg := range args {
		if flagsDone {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if arg == "--check" {
			opts.check = true
			continue
		}
		if arg == "--help" {
			opts.helpRequested = true
			return opts, nil
		}
		if arg == "--version" {
			opts.versionRequested = true
			return opts, nil
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flags: -c, or grouped.
			for _, ch := range arg[1:] {
				switch ch {
				case 'c':
					opts.check = true
				}
			}
			continue
		}
		// Not a flag — treat as file argument.
		files = append(files, arg)
	}

	return opts, files
}

// printHelp prints usage information to stdout.
// R1.4: --help prints usage to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: sha512sum [OPTION]... [FILE]...
Print or check SHA512 (512-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary  read in binary mode
  -c, --check   read SHA512 sums from the FILEs and check them
  -t, --text    read in text mode (default)
      --tag     create a BSD-style checksum
      --strict  exit non-zero for improperly formatted checksum lines
  -w, --warn    warn about improperly formatted checksum lines
      --help    display this help and exit
      --version output version information and exit
`)
}

// printVersion prints version information to stdout.
// R1.4: --version prints version information to stdout and exits 0.
func printVersion() {
	fmt.Printf("sha512sum (%s) %s\n", "go-unix-utils", version.Version)
}

// isEPIPE returns true if err wraps a syscall.EPIPE error.
func isEPIPE(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EPIPE
	}
	return false
}

// unwrapPathError extracts the inner error from an *os.PathError.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
