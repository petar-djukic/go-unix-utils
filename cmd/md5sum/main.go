// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd030-md5sum R1.1–R1.4, R2.1–R2.4, R3.1–R3.3, R4.1–R4.3
package main

import (
	"bufio"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "md5sum"

func main() {
	// R4.3: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	flagBinary := flag.Bool("b", false, "read in binary mode")
	flag.BoolVar(flagBinary, "binary", false, "read in binary mode")
	flagText := flag.Bool("t", false, "read in text mode (default)")
	flag.BoolVar(flagText, "text", false, "read in text mode (default)")
	flagTag := flag.Bool("tag", false, "create a BSD-style checksum")

	// R2.1: --check/-c flag to enter check mode.
	flagCheck := flag.Bool("c", false, "read checksums from FILEs and check them")
	flag.BoolVar(flagCheck, "check", false, "read checksums from FILEs and check them")

	// R2.3: --warn/-w prints a warning for each improperly formatted line.
	flagWarn := flag.Bool("w", false, "warn about improperly formatted checksum lines")
	flag.BoolVar(flagWarn, "warn", false, "warn about improperly formatted checksum lines")

	// R2.4: --quiet suppresses OK lines; --status suppresses all output.
	flagQuiet := flag.Bool("quiet", false, "don't print OK for each successfully verified file")
	flagStatus := flag.Bool("status", false, "don't output anything, status code shows success")

	flag.Parse()

	// R1.1: default is text mode. -b overrides to binary unless -t is also set
	// (last flag wins in GNU, but Go flag package takes the last parse; match
	// GNU behavior: -b sets binary mode).
	binaryMode := *flagBinary && !*flagText

	args := flag.Args()
	exitCode := 0

	if *flagCheck {
		// R2.1: check mode — verify checksums from files or stdin.
		opts := checkOpts{
			warn:   *flagWarn,
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

	// R4.1: exit 0 when all files processed and all digests match.
	// R4.2: exit 1 when any file cannot be opened, any digest fails, or checksum file is unreadable.
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

// hashReader computes the MD5 digest of r and prints the result line to stdout.
// R1.1: GNU format "HASH  FILENAME" (text) or "HASH *FILENAME" (binary).
// R1.3: BSD tag format "MD5 (FILENAME) = HASH" when tag is true.
func hashReader(r io.Reader, name string, binaryMode, tag bool) error {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", h.Sum(nil))

	if tag {
		// R1.3: BSD-style output.
		fmt.Printf("MD5 (%s) = %s\n", name, digest)
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
// error messages matching GNU md5sum format.
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}

// checkOpts holds the flags that modify check mode output.
type checkOpts struct {
	warn   bool // R2.3: print per-line warnings for malformed lines.
	quiet  bool // R2.4: suppress OK lines.
	status bool // R2.4: suppress all output; exit code conveys result.
}

// gnuLineRe matches GNU format: 32 hex chars followed by two spaces (text) or
// space+asterisk (binary), then the filename.
var gnuLineRe = regexp.MustCompile(`^([0-9a-fA-F]{32}) ([ *])(.+)$`)

// bsdTagRe matches BSD tag format: "MD5 (FILENAME) = HASH".
var bsdTagRe = regexp.MustCompile(`^MD5 \((.+)\) = ([0-9a-fA-F]{32})$`)

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

// computeFileHash computes the MD5 digest of the named file, returning the
// lowercase hex string. Reuses the hashing logic from hashReader (D1).
func computeFileHash(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close() // best-effort cleanup, error ignored

	h := md5.New()
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
// R2.3: --warn prints per-line warnings for malformed lines.
// R2.4: --quiet suppresses OK; --status suppresses all stdout and stderr output.
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
				// R2.3: per-line warning on stderr.
				fmt.Fprintf(os.Stderr, "%s: %s: %d: improperly formatted MD5 checksum line\n",
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
			if !opts.status && !opts.quiet {
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
		if !opts.status {
			fmt.Fprintf(os.Stderr, "%s: %s: no properly formatted checksum lines found\n",
				progName, source)
		}
		return 1
	}

	// R2.3: summary warning for malformed lines on stderr (GNU always prints this).
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

	// R2.4: exit 1 when any checksum fails or any file could not be read.
	if failCount > 0 || readErrorCount > 0 {
		return 1
	}

	return 0
}
