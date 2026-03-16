// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd031-sha1sum R1.1-R1.4, R2.1-R2.3, R3.1-R3.2: core SHA-1
// digest computation, standard GNU output format, --check verification
// mode, and --tag BSD-style output format with --binary/--text mode flags.
// Computes SHA-1 digests for files or stdin, printing one line per input
// in text, binary, or BSD tag format. In check mode, reads a checksum file
// and verifies each listed file, printing OK/FAILED. Installs SIGPIPE
// handler for clean exit on broken pipe (R4.3 prerequisite).
package main

import (
	"bufio"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
	"github.com/petar-djukic/go-unix-utils/pkg/version"
)

// progName is the name used in error messages to match GNU sha1sum format.
const progName = "sha1sum"

// sha1HexLen is the length of a SHA-1 digest in hexadecimal characters.
const sha1HexLen = 40

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])

	// R3.2: GNU sha1sum rejects --tag combined with --text.
	if opts.tagMode && opts.textSet {
		fmt.Fprintf(os.Stderr, "%s: --tag does not support --text mode\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	// D2: --tag does not support --check.
	if opts.tagMode && opts.check {
		fmt.Fprintf(os.Stderr, "%s: the --tag option is meaningless when verifying checksums\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	var exitCode int
	if opts.check {
		exitCode = runCheck(files)
	} else {
		exitCode = run(opts.binaryMode, opts.tagMode, files)
	}
	os.Exit(exitCode)
}

// options holds parsed command-line flag state.
type options struct {
	binaryMode bool
	textSet    bool // true when -t/--text was explicitly given
	check      bool
	tagMode    bool
}

// run processes files and returns the exit code.
func run(binaryMode, tagMode bool, files []string) int {
	exitCode := 0

	if len(files) == 0 {
		// R1.2: no file arguments — read from stdin.
		if err := hashReader(os.Stdin, "-", binaryMode, tagMode); err != nil {
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
			if err := hashReader(os.Stdin, "-", binaryMode, tagMode); err != nil {
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
		if err := hashReader(f, name, binaryMode, tagMode); err != nil {
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

// hashReader computes the SHA-1 digest of r and writes one output line.
// R1.1: format is "HASH  FILENAME" (text mode) or "HASH *FILENAME" (binary mode).
// R3.1/R3.2: --tag uses BSD-style "SHA1 (FILENAME) = HASH"; mode flag has no effect.
func hashReader(r io.Reader, name string, binaryMode, tagMode bool) error {
	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}

	digest := fmt.Sprintf("%x", h.Sum(nil))

	var err error
	if tagMode {
		// R3.2: BSD tag format — mode flag has no effect on output format.
		_, err = fmt.Fprintf(os.Stdout, "SHA1 (%s) = %s\n", name, digest)
	} else {
		// R3.1: text mode uses two spaces; binary mode uses space+asterisk.
		sep := "  "
		if binaryMode {
			sep = " *"
		}
		_, err = fmt.Fprintf(os.Stdout, "%s%s%s\n", digest, sep, name)
	}
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
			if line != "" {
				result.malformed++
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
// "SHA1 (FILENAME) = HASH".
// Returns the lowercase hex hash, the filename, and whether the line was valid.
func parseCheckLine(line string) (hash, filename string, ok bool) {
	// R2.2: Try BSD tag format first: "SHA1 (FILENAME) = HASH"
	if strings.HasPrefix(line, "SHA1 (") {
		return parseBSDTagLine(line)
	}

	// GNU text mode: "da39a3ee5e6b4b0d3255bfef95601890afd80709  filename"
	// GNU binary mode: "da39a3ee5e6b4b0d3255bfef95601890afd80709 *filename"
	if len(line) < sha1HexLen+2 {
		return "", "", false
	}

	hash = line[:sha1HexLen]
	if !isValidHex(hash) {
		return "", "", false
	}

	sep := line[sha1HexLen : sha1HexLen+2]
	if sep != "  " && sep != " *" {
		return "", "", false
	}

	filename = line[sha1HexLen+2:]
	if filename == "" {
		return "", "", false
	}

	return strings.ToLower(hash), filename, true
}

// parseBSDTagLine parses a BSD tag format line: "SHA1 (FILENAME) = HASH".
func parseBSDTagLine(line string) (hash, filename string, ok bool) {
	// Find the closing paren.
	closeIdx := strings.LastIndex(line, ") = ")
	if closeIdx < 6 {
		return "", "", false
	}

	filename = line[6:closeIdx]
	if filename == "" {
		return "", "", false
	}

	hash = line[closeIdx+4:]
	if len(hash) != sha1HexLen || !isValidHex(hash) {
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

// computeFileHash computes the SHA-1 hex digest of the named file.
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

	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// parseArgs separates flags from file arguments. Supports -b/--binary,
// -t/--text, -c/--check, --tag, --help, --version, and -- to end flag
// parsing. Single-char flags can be grouped.
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
		if arg == "--binary" {
			opts.binaryMode = true
			continue
		}
		if arg == "--text" {
			opts.binaryMode = false
			opts.textSet = true
			continue
		}
		if arg == "--check" {
			opts.check = true
			continue
		}
		if arg == "--tag" {
			opts.tagMode = true
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
			// Short flags: -b, -t, -c, or grouped like -bc.
			for _, ch := range arg[1:] {
				switch ch {
				case 'b':
					opts.binaryMode = true
				case 't':
					opts.binaryMode = false
					opts.textSet = true
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
func printHelp() {
	fmt.Print(`Usage: sha1sum [OPTION]... [FILE]...
Print or check SHA1 (160-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary  read in binary mode
  -c, --check   read SHA1 sums from the FILEs and check them
  -t, --text    read in text mode (default)
      --tag     create a BSD-style checksum
      --help    display this help and exit
      --version output version information and exit
`)
}

// printVersion prints version information to stdout.
func printVersion() {
	fmt.Printf("sha1sum (%s) %s\n", "go-unix-utils", version.Version)
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
