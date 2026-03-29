// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sha256sum computes SHA-256 message digests for files or stdin.
//
// Implements prd032-sha256sum: R1.1 (file digest computation), R1.2 (stdin),
// R1.3 (--tag BSD-style output), R1.4 (exit 1 on read error).
package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sha256Config is the hashutil configuration for SHA-256 digests.
//
// R1.1: SHA-256 hash via crypto/sha256 with 64-character hex digest.
var sha256Config = hashutil.HashConfig{
	Algorithm: "SHA256",
	NewHash:   sha256.New,
	DigestLen: 64,
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
	// R1.4 / R4.3: SIGPIPE handling.
	sys.InstallSIGPIPEHandler()

	opts := parseArgs(os.Args[1:])

	if err := validateCheckFlags(opts); err != nil {
		fmt.Fprintf(os.Stderr, "sha256sum: %s\n", err)
		os.Exit(1)
	}

	if opts.check {
		exitCode := runCheckMode(opts)
		os.Exit(exitCode)
	}

	// R1.1, R1.2: Compute digests for files or stdin.
	exitCode := hashutil.DigestFiles(opts.files, sha256Config, opts.binary, opts.tag, os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// validateCheckFlags returns an error if check-only flags are used without -c.
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
		allOK, err := hashutil.VerifyChecksums(f, sha256Config, checkOpts, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sha256sum: %s\n", err)
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
// R1.3: --tag enables BSD-style output.
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
			fmt.Fprintln(os.Stdout, "sha256sum (go-unix-utils)")
			os.Exit(0)
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return opts
}

// printUsage prints the usage message to stdout.
func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: sha256sum [OPTION]... [FILE]...")
	fmt.Fprintln(os.Stdout, "Print or check SHA256 checksums.")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "  -b, --binary  read in binary mode")
	fmt.Fprintln(os.Stdout, "  -t, --text    read in text mode (default)")
	fmt.Fprintln(os.Stdout, "      --tag     create a BSD-style checksum")
	fmt.Fprintln(os.Stdout, "  -c, --check   read checksums from FILEs and check them")
	fmt.Fprintln(os.Stdout, "      --quiet   don't print OK for each successfully verified file")
	fmt.Fprintln(os.Stdout, "      --status  don't output anything, status code shows success")
	fmt.Fprintln(os.Stdout, "  -w, --warn    warn about improperly formatted checksum lines")
}
