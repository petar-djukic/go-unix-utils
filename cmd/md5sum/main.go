// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd030-md5sum R1.1-R1.4, R2.1-R2.4: core MD5 digest computation,
// standard GNU output format, and --check verification mode. Computes MD5
// digests for files or stdin, printing one line per input in text or binary mode
// format. In check mode, reads a checksum file and verifies each listed file.
// Installs SIGPIPE handler for clean exit on broken pipe (R4.3).
package main

import (
	"bufio"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the name used in error messages to match GNU md5sum format.
const progName = "md5sum"

func main() {
	sys.InstallSIGPIPEHandler()

	opts, files := parseArgs(os.Args[1:])
	var exitCode int
	if opts.check {
		exitCode = runCheck(files)
	} else {
		exitCode = run(opts.binaryMode, files)
	}
	os.Exit(exitCode)
}

// options holds parsed command-line flag state.
type options struct {
	binaryMode bool
	check      bool
}

// run processes files and returns the exit code.
func run(binaryMode bool, files []string) int {
	exitCode := 0

	if len(files) == 0 {
		// R1.2: no file arguments — read from stdin.
		if err := hashReader(os.Stdin, "-", binaryMode); err != nil {
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
			if err := hashReader(os.Stdin, "-", binaryMode); err != nil {
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
		if err := hashReader(f, name, binaryMode); err != nil {
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

// hashReader computes the MD5 digest of r and writes one output line.
// R1.1: format is "HASH  FILENAME" (text mode) or "HASH *FILENAME" (binary mode).
func hashReader(r io.Reader, name string, binaryMode bool) error {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}

	digest := fmt.Sprintf("%x", h.Sum(nil))

	// R1.4/R3.1-R3.2: text mode uses two spaces; binary mode uses space+asterisk.
	sep := "  "
	if binaryMode {
		sep = " *"
	}

	_, err := fmt.Fprintf(os.Stdout, "%s%s%s\n", digest, sep, name)
	return err
}

// checkResult tracks the outcome of check mode verification.
type checkResult struct {
	mismatched int
	unreadable int
}

// runCheck reads one or more checksum files and verifies each listed file.
// R2.1: parses GNU format lines. R2.2: prints OK/FAILED. R2.3: summary warning
// on stderr. R2.4: exit 0 on all-pass, 1 on any failure.
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
			// Silently skip malformed lines (--warn not implemented in this task).
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
// D1: expects GNU format "HASH  FILENAME" or "HASH *FILENAME".
// Returns the lowercase hex hash, the filename, and whether the line was valid.
func parseCheckLine(line string) (hash, filename string, ok bool) {
	// GNU text mode: "d41d8cd98f00b204e9800998ecf8427e  filename"
	// GNU binary mode: "d41d8cd98f00b204e9800998ecf8427e *filename"
	if len(line) < 34 {
		return "", "", false
	}

	hash = line[:32]
	// Validate that hash is 32 hex characters.
	for _, ch := range hash {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return "", "", false
		}
	}

	sep := line[32:34]
	if sep != "  " && sep != " *" {
		return "", "", false
	}

	filename = line[34:]
	if filename == "" {
		return "", "", false
	}

	return strings.ToLower(hash), filename, true
}

// computeFileHash computes the MD5 hex digest of the named file.
// D2: reuses the same MD5 computation logic from R1.1-R1.4.
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

	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// parseArgs separates flags from file arguments. Supports -b/--binary,
// -t/--text, -c/--check, and -- to end flag parsing. Single-char flags can be
// grouped.
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
			continue
		}
		if arg == "--check" {
			opts.check = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && arg[1] != '-' {
			// Short flags: -b, -t, -c, or grouped like -bc.
			for _, ch := range arg[1:] {
				switch ch {
				case 'b':
					opts.binaryMode = true
				case 't':
					opts.binaryMode = false
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
