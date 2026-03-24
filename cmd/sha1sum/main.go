// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd031-sha1sum R1.1-R1.4, R2.1-R2.2: Compute and check SHA-1
// message digests using pkg/hashutil for shared formatting and verification.
package main

import (
	"crypto/sha1"
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// sha1Config is the HashConfig for SHA-1 digests.
// D1: Algorithm="SHA1", NewHash=sha1.New, DigestLen=40 (hex length).
var sha1Config = hashutil.HashConfig{
	Algorithm: "SHA1",
	NewHash:   sha1.New,
	DigestLen: 40,
}

// sha1sumOptions holds the parsed flag state for a sha1sum invocation.
type sha1sumOptions struct {
	binary bool // -b/--binary: asterisk before filename (R3.1)
	text   bool // -t/--text: default mode, two spaces (R3.1)
	tag    bool // --tag: BSD-style output (R1.3)
	check  bool // -c/--check: verify checksums (R2.1)
	warn   bool // -w/--warn: warn on malformed lines (R2.3)
	quiet  bool // --quiet: suppress OK lines (R2.3)
	status bool // --status: suppress all output (R2.3)
}

func main() {
	// R4.3: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, files := parseFlags(os.Args[1:])
	exitCode := run(opts, files)
	os.Exit(exitCode)
}

// run executes the sha1sum command with the given options and files.
// D2: delegates to hashutil.DigestFiles for digest mode,
// hashutil.VerifyChecksums for check mode.
func run(opts sha1sumOptions, files []string) int {
	if opts.check {
		return runCheck(opts, files)
	}
	return hashutil.DigestFiles(files, sha1Config, opts.binary, opts.tag, os.Stdout, os.Stderr)
}

// runCheck verifies checksums from the given files.
// R2.1-R2.2: reads checksum file, prints OK/FAILED per entry.
func runCheck(opts sha1sumOptions, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	checkOpts := hashutil.CheckOptions{
		Warn:   opts.warn,
		Quiet:  opts.quiet,
		Status: opts.status,
	}
	exitCode := 0
	for _, f := range files {
		ok, err := hashutil.VerifyChecksums(f, sha1Config, checkOpts, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sha1sum: %v\n", err)
			exitCode = 1
		} else if !ok {
			exitCode = 1
		}
	}
	return exitCode
}

// parseFlags parses GNU sha1sum-compatible flags from the argument list.
// D4: uses manual parsing for both short and long flag forms.
func parseFlags(args []string) (sha1sumOptions, []string) {
	var opts sha1sumOptions
	var files []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if handled := handleLongFlag(arg, &opts); handled {
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			if err := applyShortFlags(arg[1:], &opts); err != nil {
				fmt.Fprintf(os.Stderr, "sha1sum: %s\n", err)
				os.Exit(1)
			}
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// handleLongFlag processes a long-form flag and returns true if handled.
func handleLongFlag(arg string, opts *sha1sumOptions) bool {
	switch arg {
	case "--binary":
		opts.binary = true
	case "--text":
		opts.text = true
	case "--tag":
		opts.tag = true
	case "--check":
		opts.check = true
	case "--warn":
		opts.warn = true
	case "--quiet":
		opts.quiet = true
	case "--status":
		opts.status = true
	case "--version":
		fmt.Printf("sha1sum (go-unix-utils) %s\n", version)
		os.Exit(0)
	case "--help":
		printHelp()
		os.Exit(0)
	default:
		return false
	}
	return true
}

// applyShortFlags applies a sequence of short flag characters to opts.
func applyShortFlags(flags string, opts *sha1sumOptions) error {
	for _, c := range flags {
		switch c {
		case 'b':
			opts.binary = true
		case 't':
			opts.text = true
		case 'c':
			opts.check = true
		case 'w':
			opts.warn = true
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: sha1sum [OPTION]... [FILE]...
Print or check SHA1 (160-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary   read in binary mode
  -c, --check    read checksums from the FILEs and check them
      --tag      create a BSD-style checksum
  -t, --text     read in text mode (default)
  -w, --warn     warn about improperly formatted checksum lines
      --quiet    don't print OK for each successfully verified file
      --status   don't output anything, status code shows success
      --help     display this help and exit
      --version  output version information and exit
`)
}
