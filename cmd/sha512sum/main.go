// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd033-sha512sum R1.1–R1.4, R2.1–R2.3
package main

import (
	"bufio"
	"crypto/sha512"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "sha512sum"

func main() {
	// R1.4/R4.3: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	// Use ContinueOnError for GNU-compatible error handling.
	flag.CommandLine.Init(os.Args[0], flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	flagVersion := flag.Bool("version", false, "output version information and exit")
	flagBinary := flag.Bool("b", false, "read in binary mode")
	flag.BoolVar(flagBinary, "binary", false, "read in binary mode")
	flagText := flag.Bool("t", false, "read in text mode (default)")
	flag.BoolVar(flagText, "text", false, "read in text mode (default)")
	flagTag := flag.Bool("tag", false, "create a BSD-style checksum")

	// R2.1: --check/-c flag to enter check mode.
	flagCheck := flag.Bool("c", false, "read checksums from FILEs and check them")
	flag.BoolVar(flagCheck, "check", false, "read checksums from FILEs and check them")

	// R2.2: --warn/-w prints a warning for each improperly formatted line.
	flagWarn := flag.Bool("w", false, "warn about improperly formatted checksum lines")
	flag.BoolVar(flagWarn, "warn", false, "warn about improperly formatted checksum lines")

	// R2.3: --strict exits non-zero for improperly formatted checksum lines.
	flagStrict := flag.Bool("strict", false, "exit non-zero for improperly formatted checksum lines")

	// R2.2: --quiet suppresses OK lines; --status suppresses all output.
	flagQuiet := flag.Bool("quiet", false, "don't print OK for each successfully verified file")
	flagStatus := flag.Bool("status", false, "don't output anything, status code shows success")

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			printUsage()
			return
		}
		fmt.Fprintf(os.Stderr, "%s: %v\nTry '%s --help' for more information.\n", progName, err, progName)
		os.Exit(1)
	}

	if *flagVersion {
		fmt.Printf("%s (go-unix-utils) 1.0\n", progName)
		return
	}

	// R1.1: default is text mode. -b overrides to binary unless -t is also set.
	binaryMode := *flagBinary && !*flagText

	args := flag.Args()
	exitCode := 0

	if *flagCheck {
		// R2.1: check mode — verify checksums from files or stdin.
		opts := checkOpts{
			warn:   *flagWarn,
			strict: *flagStrict,
			quiet:  *flagQuiet,
			status: *flagStatus,
		}
		if len(args) == 0 {
			exitCode = checkChecksums(os.Stdin, "-", opts)
		} else {
			for _, name := range args {
				var code int
				if name == "-" {
					code = checkChecksums(os.Stdin, "-", opts)
				} else {
					f, err := os.Open(name)
					if err != nil {
						fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, unwrapPathError(err))
						exitCode = 1
						continue
					}
					code = checkChecksums(f, name, opts)
					f.Close() // best-effort cleanup, error ignored
				}
				if code != 0 {
					exitCode = 1
				}
			}
		}
	} else if len(args) == 0 {
		// R1.2: no file arguments — read from stdin.
		if err := hashReader(os.Stdin, "-", binaryMode, *flagTag); err != nil {
			fmt.Fprintf(os.Stderr, "%s: -: %v\n", progName, err)
			exitCode = 1
		}
	} else {
		for _, name := range args {
			if name == "-" {
				// R1.2: "-" means stdin.
				if err := hashReader(os.Stdin, "-", binaryMode, *flagTag); err != nil {
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
			if err := hashReader(f, name, binaryMode, *flagTag); err != nil {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, name, err)
				exitCode = 1
			}
			f.Close() // best-effort cleanup, error ignored
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// hashReader computes the SHA-512 digest of r and prints the result line to stdout.
// R1.1: GNU format "HASH  FILENAME" (text) or "HASH *FILENAME" (binary).
// R1.3: BSD tag format "SHA512 (FILENAME) = HASH" when tag is true.
func hashReader(r io.Reader, name string, binaryMode, tag bool) error {
	h := sha512.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", h.Sum(nil))

	if tag {
		// R1.3: BSD-style output.
		fmt.Printf("SHA512 (%s) = %s\n", name, digest)
	} else if binaryMode {
		// R1.1: binary mode — "HASH *FILENAME".
		fmt.Printf("%s *%s\n", digest, name)
	} else {
		// R1.1: text mode (default) — "HASH  FILENAME" (two spaces).
		fmt.Printf("%s  %s\n", digest, name)
	}
	return nil
}

// unwrapPathError extracts the inner error from an os.PathError for cleaner
// error messages matching GNU sha512sum format.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// printUsage writes GNU-compatible usage information to stdout.
func printUsage() {
	fmt.Printf("Usage: %s [OPTION]... [FILE]...\n", progName)
	fmt.Printf("Print or check SHA512 (512-bit) checksums.\n\n")
	fmt.Printf("With no FILE, or when FILE is -, read standard input.\n\n")
	fmt.Printf("  -b, --binary         read in binary mode\n")
	fmt.Printf("  -c, --check          read checksums from FILEs and check them\n")
	fmt.Printf("      --tag            create a BSD-style checksum\n")
	fmt.Printf("  -t, --text           read in text mode (default)\n")
	fmt.Printf("\nThe following five options are useful only when verifying checksums:\n")
	fmt.Printf("      --quiet          don't print OK for each successfully verified file\n")
	fmt.Printf("      --status         don't output anything, status code shows success\n")
	fmt.Printf("      --strict         exit non-zero for improperly formatted checksum lines\n")
	fmt.Printf("  -w, --warn           warn about improperly formatted checksum lines\n")
	fmt.Printf("\n      --help     display this help and exit\n")
	fmt.Printf("      --version  output version information and exit\n")
}

// checkOpts holds the flags that modify check mode output.
type checkOpts struct {
	warn   bool // R2.2: print per-line warnings for malformed lines.
	strict bool // R2.3: exit non-zero for improperly formatted checksum lines.
	quiet  bool // R2.2: suppress OK lines.
	status bool // R2.2: suppress all output; exit code conveys result.
}

// gnuLineRe matches GNU format: 128 hex chars followed by two spaces (text) or
// space+asterisk (binary), then the filename.
var gnuLineRe = regexp.MustCompile(`^([0-9a-fA-F]{128}) ([ *])(.+)$`)

// bsdTagRe matches BSD tag format: "SHA512 (FILENAME) = HASH".
var bsdTagRe = regexp.MustCompile(`^SHA512 \((.+)\) = ([0-9a-fA-F]{128})$`)

// parsedLine holds the result of parsing one checksum line.
type parsedLine struct {
	hash     string
	filename string
}

// parseChecksumLine parses a single line in GNU or BSD tag format.
// Returns the parsed result and true on success, or zero value and false for
// malformed lines.
//
// R2.1: Parse GNU format "HASH  FILENAME" / "HASH *FILENAME" and BSD tag format.
func parseChecksumLine(line string) (parsedLine, bool) {
	if m := bsdTagRe.FindStringSubmatch(line); m != nil {
		return parsedLine{hash: strings.ToLower(m[2]), filename: m[1]}, true
	}
	if m := gnuLineRe.FindStringSubmatch(line); m != nil {
		return parsedLine{hash: strings.ToLower(m[1]), filename: m[3]}, true
	}
	return parsedLine{}, false
}

// computeFileHash computes the SHA-512 digest of the named file, returning the
// lowercase hex string.
func computeFileHash(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close() // best-effort cleanup, error ignored

	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// checkChecksums reads checksum lines from r and verifies each one.
// Returns 0 if all checksums match and no errors occur, 1 otherwise.
//
// R2.1: Read and parse each line.
// R2.2: Print "FILENAME: OK" or "FILENAME: FAILED".
// R2.2: --warn prints per-line warnings for malformed lines.
// R2.3: --strict exits non-zero for improperly formatted checksum lines.
func checkChecksums(r io.Reader, source string, opts checkOpts) int {
	scanner := bufio.NewScanner(r)
	var failCount int
	var readErrorCount int
	var malformedCount int
	var validCount int
	var lineNum int

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		parsed, ok := parseChecksumLine(line)
		if !ok {
			malformedCount++
			if opts.warn {
				// R2.2: per-line warning on stderr.
				fmt.Fprintf(os.Stderr, "%s: %s: %d: improperly formatted SHA512 checksum line\n",
					progName, source, lineNum)
			}
			continue
		}
		validCount++

		computed, err := computeFileHash(parsed.filename)
		if err != nil {
			// File open/read failure — separate from hash mismatch.
			readErrorCount++
			if !opts.status {
				fmt.Fprintf(os.Stderr, "%s: %s: %v\n", progName, parsed.filename, unwrapPathError(err))
				fmt.Printf("%s: FAILED open or read\n", parsed.filename)
			}
			continue
		}

		if computed == parsed.hash {
			if !opts.status && (!opts.quiet || opts.warn) {
				// R2.2: print OK.
				fmt.Printf("%s: OK\n", parsed.filename)
			}
		} else {
			failCount++
			if !opts.status {
				// R2.2: print FAILED.
				fmt.Printf("%s: FAILED\n", parsed.filename)
			}
		}
	}

	// Check if no valid lines were found at all.
	if validCount == 0 {
		fmt.Fprintf(os.Stderr, "%s: %s: no properly formatted checksum lines found\n",
			progName, source)
		return 1
	}

	// R2.2: summary warning for malformed lines on stderr.
	if malformedCount > 0 && !opts.status {
		if malformedCount == 1 {
			fmt.Fprintf(os.Stderr, "%s: WARNING: %d line is improperly formatted\n",
				progName, malformedCount)
		} else {
			fmt.Fprintf(os.Stderr, "%s: WARNING: %d lines are improperly formatted\n",
				progName, malformedCount)
		}
	}
	// Summary for files that could not be read.
	if readErrorCount > 0 && !opts.status {
		if readErrorCount == 1 {
			fmt.Fprintf(os.Stderr, "%s: WARNING: %d listed file could not be read\n",
				progName, readErrorCount)
		} else {
			fmt.Fprintf(os.Stderr, "%s: WARNING: %d listed files could not be read\n",
				progName, readErrorCount)
		}
	}
	// Summary for hash mismatches.
	if failCount > 0 && !opts.status {
		if failCount == 1 {
			fmt.Fprintf(os.Stderr, "%s: WARNING: %d computed checksum did NOT match\n",
				progName, failCount)
		} else {
			fmt.Fprintf(os.Stderr, "%s: WARNING: %d computed checksums did NOT match\n",
				progName, failCount)
		}
	}

	// R2.3: --strict exits non-zero when any line is malformed.
	if opts.strict && malformedCount > 0 {
		return 1
	}

	// Exit 1 when any checksum fails or any file could not be read.
	if failCount > 0 || readErrorCount > 0 {
		return 1
	}

	return 0
}
