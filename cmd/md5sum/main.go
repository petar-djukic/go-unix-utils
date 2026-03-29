// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/md5sum computes MD5 message digests for files or stdin.
//
// Implements prd030-md5sum: R1.1 (file digest computation), R1.2 (stdin),
// R1.3 (binary/text mode flags), R1.4 (SIGPIPE handling),
// R2.1 (--check), R2.2 (--quiet), R2.3 (--status), R2.4 (--warn).
package main

import (
	"crypto/md5"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// md5Config is the hashutil configuration for MD5 digests.
//
// R1.1: MD5 hash via crypto/md5 with 32-character hex digest.
var md5Config = hashutil.HashConfig{
	Algorithm: "MD5",
	NewHash:   md5.New,
	DigestLen: 32,
}

// options holds all parsed command-line flags.
type options struct {
	binary bool
	tag    bool
	check  bool
	quiet  bool
	status bool
	warn   bool
	files  []string
}

func main() {
	// R1.4, R4.3: SIGPIPE handling.
	sys.InstallSIGPIPEHandler()

	opts := parseArgs(os.Args[1:])

	if err := validateCheckFlags(opts); err != nil {
		fmt.Fprintf(os.Stderr, "md5sum: %s\n", err)
		os.Exit(1)
	}

	if opts.check {
		exitCode := runCheckMode(opts)
		os.Exit(exitCode)
	}

	exitCode := hashutil.DigestFiles(opts.files, md5Config, opts.binary, opts.tag, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// validateCheckFlags returns an error if check-only flags are used without -c.
//
// D3: --quiet, --status, --warn are only valid with -c.
func validateCheckFlags(opts options) error {
	if opts.check {
		return nil
	}
	if opts.quiet {
		return fmt.Errorf("the --quiet option is meaningful only when verifying checksums")
	}
	if opts.status {
		return fmt.Errorf("the --status option is meaningful only when verifying checksums")
	}
	if opts.warn {
		return fmt.Errorf("the --warn option is meaningful only when verifying checksums")
	}
	return nil
}

// runCheckMode verifies checksums from files and returns the exit code.
//
// R2.1: -c reads checksum file and verifies entries via hashutil.VerifyChecksums.
func runCheckMode(opts options) int {
	checkOpts := hashutil.CheckOptions{
		Quiet:  opts.quiet,
		Status: opts.status,
		Warn:   opts.warn,
	}
	files := opts.files
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, f := range files {
		allOK, err := hashutil.VerifyChecksums(f, md5Config, checkOpts, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "md5sum: %s\n", err)
			exitCode = 1
			continue
		}
		if !allOK {
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs parses GNU-compatible flags from args and returns the parsed options.
//
// R1.3: -b/--binary sets binary mode; -t/--text sets text mode (default).
// R2.1: -c/--check enables check mode.
// R2.2: --quiet suppresses OK lines. R2.3: --status suppresses all output.
// R2.4: -w/--warn warns about malformed lines.
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
		case "-c", "--check":
			opts.check = true
		case "--quiet":
			opts.quiet = true
		case "--status":
			opts.status = true
		case "-w", "--warn":
			opts.warn = true
		case "--":
			continue
		case "--help":
			printUsage()
			os.Exit(0)
		case "--version":
			fmt.Fprintln(os.Stdout, "md5sum (go-unix-utils)")
			os.Exit(0)
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return opts
}

// printUsage prints the usage message to stdout.
func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: md5sum [OPTION]... [FILE]...")
	fmt.Fprintln(os.Stdout, "Print or check MD5 checksums.")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "  -b, --binary  read in binary mode")
	fmt.Fprintln(os.Stdout, "  -t, --text    read in text mode (default)")
	fmt.Fprintln(os.Stdout, "      --tag     create a BSD-style checksum")
	fmt.Fprintln(os.Stdout, "  -c, --check   read checksums from FILEs and check them")
	fmt.Fprintln(os.Stdout, "      --quiet   don't print OK for each successfully verified file")
	fmt.Fprintln(os.Stdout, "      --status  don't output anything, status code shows success")
	fmt.Fprintln(os.Stdout, "  -w, --warn    warn about improperly formatted checksum lines")
}
