// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/sha224sum computes SHA-224 message digests for files or stdin.
//
// Implements prd074-sha224sum: R1.1 (file digest computation), R1.2 (stdin),
// R1.3 (--tag BSD-style output), R1.4 (exit 1 on read error),
// R2.1 (--check verification), R2.2 (OK/FAILED output), R2.3 (--warn/--quiet/--status),
// R3.1 (binary mode indicator), R3.2 (--tag overrides -b/-t),
// R4.1 (--strict malformed line failure), R4.2 (--version/--help),
// R4.3 (SIGPIPE handling).
package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// sha224Config is the hashutil configuration for SHA-224 digests.
//
// R1.1: SHA-224 hash via crypto/sha256.New224 with 56-character hex digest.
var sha224Config = hashutil.HashConfig{
	Algorithm: "SHA224",
	NewHash:   sha256.New224,
	DigestLen: 56,
}

// options holds all parsed command-line flags.
type options struct {
	binary bool
	tag    bool
	check  bool
	quiet  bool
	status bool
	warn   bool
	strict bool
	files  []string
}

func main() {
	// R4.3: SIGPIPE handling.
	sys.InstallSIGPIPEHandler()

	opts := parseArgs(os.Args[1:])

	if err := validateFlags(opts); err != nil {
		fmt.Fprintf(os.Stderr, "sha224sum: %s\n", err)
		os.Exit(1)
	}

	if opts.check {
		os.Exit(runCheckMode(opts))
	}

	// R1.1, R1.2, R3.2: Compute digests for files or stdin.
	os.Exit(hashutil.DigestFiles(opts.files, sha224Config, opts.binary, opts.tag, os.Stdout, os.Stderr))
}

// validateFlags returns an error if flags are used in invalid combinations.
//
// R4.1: --strict requires --check.
// R4.3: --tag with --check is meaningless.
func validateFlags(opts options) error {
	if opts.check {
		if opts.tag {
			return fmt.Errorf("the --tag option is meaningless when verifying checksums")
		}
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
	if opts.strict {
		return fmt.Errorf("the --strict option is meaningful only when verifying checksums")
	}
	return nil
}

// runCheckMode verifies checksums from files and returns the exit code.
//
// R2.1: -c reads checksum file and verifies entries via hashutil.VerifyChecksums.
// R4.1: --strict causes non-zero exit on malformed lines.
func runCheckMode(opts options) int {
	checkOpts := hashutil.CheckOptions{
		Quiet:  opts.quiet,
		Status: opts.status,
		Warn:   opts.warn,
		Strict: opts.strict,
	}
	files := opts.files
	if len(files) == 0 {
		files = []string{"-"}
	}
	exitCode := 0
	for _, f := range files {
		allOK, err := hashutil.VerifyChecksums(f, sha224Config, checkOpts, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sha224sum: %s\n", err)
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
// R3.2: --tag enables BSD-style output regardless of -b/-t.
// R4.2: --help and --version print info and exit 0.
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
		case "--strict":
			opts.strict = true
		case "--":
			continue
		case "--help":
			printUsage()
			os.Exit(0)
		case "--version":
			fmt.Fprintln(os.Stdout, "sha224sum (go-unix-utils)")
			os.Exit(0)
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return opts
}

// printUsage prints the usage message to stdout.
//
// R4.2: --help output.
func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: sha224sum [OPTION]... [FILE]...")
	fmt.Fprintln(os.Stdout, "Print or check SHA224 checksums.")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "  -b, --binary  read in binary mode")
	fmt.Fprintln(os.Stdout, "  -t, --text    read in text mode (default)")
	fmt.Fprintln(os.Stdout, "      --tag     create a BSD-style checksum")
	fmt.Fprintln(os.Stdout, "  -c, --check   read checksums from FILEs and check them")
	fmt.Fprintln(os.Stdout, "      --quiet   don't print OK for each successfully verified file")
	fmt.Fprintln(os.Stdout, "      --status  don't output anything, status code shows success")
	fmt.Fprintln(os.Stdout, "  -w, --warn    warn about improperly formatted checksum lines")
	fmt.Fprintln(os.Stdout, "      --strict  exit non-zero for improperly formatted checksum lines")
}
