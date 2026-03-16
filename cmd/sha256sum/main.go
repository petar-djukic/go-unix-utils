// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd032-sha256sum R1.1-R1.4, R2.1-R2.3: core SHA-256 digest
// computation, standard GNU output format, stdin reading, multiple file
// processing with error handling, and --check verification mode with status
// output. Computes SHA-256 digests for files or stdin, printing one line per
// input in text mode (default). In check mode, reads a checksum file and
// verifies each listed file. Installs SIGPIPE handler for clean exit on
// broken pipe.
package main

import (
	"bufio"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU sha256sum format.
const progName = "sha256sum"

// sha256HexLen is the length of a SHA-256 digest in hexadecimal characters.
const sha256HexLen = 64

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	var exitCode int
	if opts.check {
		exitCode = runCheck(opts.warn, files)
	} else {
		exitCode = run(files)
	}
	os.Exit(exitCode)
}

// options holds parsed command-line flag state.
type options struct {
	check bool
	warn  bool // R2.3: emit stderr warning for each improperly formatted check line
}

// run processes files and returns the exit code.
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
			// R1.4: print error to stderr, continue processing remaining files.
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

// hashReader computes the SHA-256 digest of r and writes one output line.
// R1.1: format is "HASH  FILENAME" (text mode, default).
func hashReader(r io.Reader, name string) error {
	h := sha256.New()
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
	malformed  int
}

// runCheck reads one or more checksum files and verifies each listed file.
// R2.1: parses GNU and BSD format lines. R2.2: prints OK/FAILED.
// R2.3: summary warning on stderr. Exit 0 on all-pass, 1 on any failure.
func runCheck(warn bool, files []string) int {
	if len(files) == 0 {
		// --check with no file argument reads from stdin.
		files = []string{"-"}
	}

	var total checkResult
	exitCode := 0

	for _, name := range files {
		result, err := checkFile(name, warn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
			exitCode = 1
			continue
		}
		total.mismatched += result.mismatched
		total.unreadable += result.unreadable
		total.malformed += result.malformed
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
	if total.malformed > 0 {
		fmt.Fprintf(os.Stderr, "%s: WARNING: %d line is improperly formatted\n", progName, total.malformed)
	}

	return exitCode
}

// checkFile opens a checksum file (or stdin for "-"), parses each line, verifies
// the digest, and prints OK/FAILED. Returns counts of mismatches, unreadable files,
// and malformed lines.
func checkFile(name string, warn bool) (checkResult, error) {
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
	lineNum := 0
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		hash, filename, ok := parseCheckLine(line)
		if !ok {
			if line != "" {
				result.malformed++
				if warn {
					fmt.Fprintf(os.Stderr, "%s: %s: %d: improperly formatted SHA256 checksum line\n", progName, name, lineNum)
				}
			}
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
// "SHA256 (FILENAME) = HASH".
// Returns the lowercase hex hash, the filename, and whether the line was valid.
func parseCheckLine(line string) (hash, filename string, ok bool) {
	// R2.1: Try BSD tag format first: "SHA256 (FILENAME) = HASH"
	if strings.HasPrefix(line, "SHA256 (") {
		return parseBSDTagLine(line)
	}

	// GNU text mode: "hash  filename"
	// GNU binary mode: "hash *filename"
	if len(line) < sha256HexLen+2 {
		return "", "", false
	}

	hash = line[:sha256HexLen]
	if !isValidHex(hash) {
		return "", "", false
	}

	sep := line[sha256HexLen : sha256HexLen+2]
	if sep != "  " && sep != " *" {
		return "", "", false
	}

	filename = line[sha256HexLen+2:]
	if filename == "" {
		return "", "", false
	}

	return strings.ToLower(hash), filename, true
}

// parseBSDTagLine parses a BSD tag format line: "SHA256 (FILENAME) = HASH".
func parseBSDTagLine(line string) (hash, filename string, ok bool) {
	// "SHA256 (" is 8 characters.
	closeIdx := strings.LastIndex(line, ") = ")
	if closeIdx < 8 {
		return "", "", false
	}

	filename = line[8:closeIdx]
	if filename == "" {
		return "", "", false
	}

	hash = line[closeIdx+4:]
	if len(hash) != sha256HexLen || !isValidHex(hash) {
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

// computeFileHash computes the SHA-256 hex digest of the named file.
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

	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// parseArgs separates flags from file arguments. Supports -c/--check,
// -w/--warn, --help, --version, and -- to end flag parsing.
// Single-char flags can be grouped.
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
		if arg == "--warn" {
			opts.warn = true
			continue
		}
		if arg == "--help" {
			printHelp()
			os.Exit(0)
		}
		if arg == "--version" {
			printVersion()
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flags: -c, -w, or grouped like -cw.
			for _, ch := range arg[1:] {
				switch ch {
				case 'c':
					opts.check = true
				case 'w':
					opts.warn = true
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
func printHelp() {
	fmt.Print(`Usage: sha256sum [OPTION]... [FILE]...
Print or check SHA256 (256-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary  read in binary mode
  -c, --check   read SHA256 sums from the FILEs and check them
  -t, --text    read in text mode (default)
      --tag     create a BSD-style checksum
      --strict  exit non-zero for improperly formatted checksum lines
  -w, --warn    warn about improperly formatted checksum lines
      --help    display this help and exit
      --version output version information and exit
`)
}

// printVersion prints version information to stdout.
func printVersion() {
	fmt.Printf("sha256sum (%s) %s\n", "go-unix-utils", version.Version)
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
