// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/b2sum computes BLAKE2b message digests for files or stdin.
//
// Implements prd076-b2sum: R1.1 (file digest computation),
// R1.2 (stdin as '-'), R1.3 (--tag with bit length in tag name),
// R1.4 (exit code and stderr diagnostics),
// R2.1 (--check verification), R2.2 (OK/FAILED output),
// R2.3 (--warn/--quiet/--status modifiers),
// R3.1 (binary/text mode), R3.2 (--tag overrides -b/-t),
// R3.3 (--length variable digest length),
// R4.1 (exit 0 on success), R4.2 (exit 1 on failure).
package main

import (
	"fmt"
	"hash"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/blake2b"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	defaultLengthBits = 512
	maxLengthBits     = 512
)

// options holds all parsed command-line flags.
type options struct {
	binary       bool
	textExplicit bool // true when -t/--text was explicitly given
	tag          bool
	check        bool
	quiet        bool
	status       bool
	warn         bool
	strict       bool
	length       int // digest length in bits (default 512)
	files        []string
}

func main() {
	// R4.3: SIGPIPE handling.
	sys.InstallSIGPIPEHandler()

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "b2sum: %s\n", err)
		os.Exit(1)
	}

	if err := validateFlags(opts); err != nil {
		fmt.Fprintf(os.Stderr, "b2sum: %s\n", err)
		os.Exit(1)
	}

	cfg := buildConfig(opts.length)

	if opts.check {
		os.Exit(runCheckMode(opts, cfg))
	}

	os.Exit(hashutil.DigestFiles(
		opts.files, cfg, opts.binary, opts.tag, os.Stdout, os.Stderr,
	))
}

// buildConfig creates a HashConfig for BLAKE2b with the given bit length.
//
// R1.3: tag name includes bit length when non-default (e.g., "BLAKE2b-256").
// R3.3: variable digest length via --length.
func buildConfig(lengthBits int) hashutil.HashConfig {
	sizeBytes := lengthBits / 8
	algo := "BLAKE2b"
	if lengthBits != defaultLengthBits {
		algo = fmt.Sprintf("BLAKE2b-%d", lengthBits)
	}
	return hashutil.HashConfig{
		Algorithm: algo,
		NewHash: func() hash.Hash {
			h, _ := blake2b.New(sizeBytes, nil) // size validated in parseArgs
			return h
		},
		DigestLen: sizeBytes * 2, // hex encoding doubles byte count
	}
}

// validateFlags returns an error if flags are used in invalid combinations.
//
// R3.2: --tag does not support --text mode (explicit -t/--text).
func validateFlags(opts options) error {
	if opts.tag && opts.textExplicit {
		return fmt.Errorf("--tag does not support --text mode")
	}
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
	if opts.strict {
		return fmt.Errorf("the --strict option is meaningful only when verifying checksums")
	}
	return nil
}

// runCheckMode verifies checksums from files and returns the exit code.
//
// R2.1: -c reads checksum file and verifies entries.
func runCheckMode(opts options, cfg hashutil.HashConfig) int {
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
		allOK, err := hashutil.VerifyChecksums(
			f, cfg, checkOpts, os.Stdout, os.Stderr,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "b2sum: %s\n", err)
			exitCode = 1
			continue
		}
		if !allOK {
			exitCode = 1
		}
	}
	return exitCode
}

// parseArgs parses GNU-compatible flags from args and returns the options.
//
// R3.1: -b/--binary sets binary mode; -t/--text sets text mode (default).
// R1.3: --tag enables BSD-style output.
// R3.3: --length=N or -l N sets digest length in bits.
func parseArgs(args []string) (options, error) {
	var opts options
	opts.length = defaultLengthBits
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-b" || arg == "--binary":
			opts.binary = true
		case arg == "-t" || arg == "--text":
			opts.binary = false
			opts.textExplicit = true
		case arg == "--tag":
			opts.tag = true
		case arg == "-c" || arg == "--check":
			opts.check = true
		case arg == "--quiet":
			opts.quiet = true
		case arg == "--status":
			opts.status = true
		case arg == "-w" || arg == "--warn":
			opts.warn = true
		case arg == "--strict":
			opts.strict = true
		case strings.HasPrefix(arg, "--length="):
			n, err := parseLengthValue(strings.TrimPrefix(arg, "--length="))
			if err != nil {
				return options{}, err
			}
			opts.length = n
		case arg == "--length" || arg == "-l":
			i++
			if i >= len(args) {
				return options{}, fmt.Errorf("option '%s' requires an argument", arg)
			}
			n, err := parseLengthValue(args[i])
			if err != nil {
				return options{}, err
			}
			opts.length = n
		case arg == "--":
			continue
		case arg == "--help":
			printUsage()
			os.Exit(0)
		case arg == "--version":
			fmt.Fprintln(os.Stdout, "b2sum (go-unix-utils)")
			os.Exit(0)
		default:
			opts.files = append(opts.files, arg)
		}
	}
	return opts, nil
}

// parseLengthValue validates and returns the digest length in bits.
//
// R3.3: N must be a non-negative multiple of 8 and at most 512.
// Zero means default (512), matching GNU b2sum behavior.
func parseLengthValue(val string) (int, error) {
	n, err := strconv.Atoi(val)
	if err != nil || n < 0 || n%8 != 0 || n > maxLengthBits {
		return 0, fmt.Errorf("invalid length: %s", val)
	}
	if n == 0 {
		return defaultLengthBits, nil
	}
	return n, nil
}

// printUsage prints the usage message to stdout.
func printUsage() {
	fmt.Fprintln(os.Stdout, "Usage: b2sum [OPTION]... [FILE]...")
	fmt.Fprintln(os.Stdout, "Print or check BLAKE2 checksums.")
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "  -b, --binary    read in binary mode")
	fmt.Fprintln(os.Stdout, "  -t, --text      read in text mode (default)")
	fmt.Fprintln(os.Stdout, "      --tag       create a BSD-style checksum")
	fmt.Fprintln(os.Stdout, "  -l, --length    digest length in bits (default 512)")
	fmt.Fprintln(os.Stdout, "  -c, --check     read checksums from FILEs and check them")
	fmt.Fprintln(os.Stdout, "      --quiet     don't print OK for each successfully verified file")
	fmt.Fprintln(os.Stdout, "      --status    don't output anything, status code shows success")
	fmt.Fprintln(os.Stdout, "  -w, --warn      warn about improperly formatted checksum lines")
	fmt.Fprintln(os.Stdout, "      --strict    exit non-zero for improperly formatted checksum lines")
}
