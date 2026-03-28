// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd033-sha512sum R1.1-R1.4, R2.1-R2.3, R3.1-R3.2: Compute and
// check SHA-512 message digests using pkg/hashutil for shared formatting and
// verification. Check mode supports --warn, --quiet, --status, and --strict.
package main

import (
	"bytes"
	"crypto/sha512"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// programName is the name used in error messages.
const programName = "sha512sum"

// sha512Config is the HashConfig for SHA-512 digests.
// R1.1: Algorithm="SHA512", NewHash=sha512.New, DigestLen=128 (hex length).
var sha512Config = hashutil.HashConfig{
	Algorithm: "SHA512",
	NewHash:   sha512.New,
	DigestLen: 128,
}

// sha512sumOptions holds the parsed flag state for a sha512sum invocation.
type sha512sumOptions struct {
	binary bool // -b/--binary: asterisk before filename (R1.3)
	text   bool // -t/--text: default mode, two spaces (R1.3)
	tag    bool // --tag: BSD-style output (R1.3)
	check  bool // -c/--check: verify checksums (R2.1)
	warn   bool // -w/--warn: warn on malformed lines (R2.3)
	quiet  bool // --quiet: suppress OK lines (R2.3)
	status bool // --status: suppress all output (R2.3)
	strict bool // --strict: exit non-zero on malformed lines (R2.3)
	zero   bool // -z/--zero: NUL terminator instead of newline
}

func main() {
	// R4.3: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, files := parseFlags(os.Args[1:])
	exitCode := run(opts, files)
	os.Exit(exitCode)
}

// run executes the sha512sum command with the given options and files.
// R1.1-R1.2: delegates to hashutil.DigestFiles for digest mode,
// hashutil.VerifyChecksums for check mode.
// R3.2: validates flag combinations before proceeding.
func run(opts sha512sumOptions, files []string) int {
	if err := validateFlags(opts); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}
	if opts.check {
		return runCheck(opts, files)
	}
	stdout := wrapStdout(opts.zero)
	return hashutil.DigestFiles(
		files, sha512Config, opts.binary, opts.tag, stdout, os.Stderr,
	)
}

// validateFlags checks for invalid flag combinations.
// R3.2: --tag is meaningless when verifying checksums.
func validateFlags(opts sha512sumOptions) error {
	if opts.tag && opts.check {
		return fmt.Errorf(
			"the --tag option is meaningless when verifying checksums",
		)
	}
	return nil
}

// wrapStdout returns a writer that replaces newlines with NUL bytes
// when zero is true, or os.Stdout otherwise.
func wrapStdout(zero bool) io.Writer {
	if zero {
		return &nulWriter{w: os.Stdout}
	}
	return os.Stdout
}

// runCheck verifies checksums from the given files.
// R2.1-R2.3: reads checksum file, prints OK/FAILED per entry.
// Supports --quiet, --status, --warn, --strict options.
func runCheck(opts sha512sumOptions, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	checkOpts := buildCheckOptions(opts)
	exitCode := 0
	for _, f := range files {
		if err := verifyOneFile(f, checkOpts, &exitCode); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// verifyOneFile runs VerifyChecksums on a single checksum file and
// updates exitCode if verification fails.
func verifyOneFile(f string, checkOpts hashutil.CheckOptions, exitCode *int) error {
	ok, err := hashutil.VerifyChecksums(
		f, sha512Config, checkOpts, os.Stdout, os.Stderr,
	)
	if err != nil {
		return err
	}
	if !ok {
		*exitCode = 1
	}
	return nil
}

// buildCheckOptions constructs CheckOptions from sha512sumOptions.
// R2.3: --strict enables warnings and is handled at the exit-code level.
func buildCheckOptions(opts sha512sumOptions) hashutil.CheckOptions {
	return hashutil.CheckOptions{
		Warn:   opts.warn || opts.strict,
		Quiet:  opts.quiet,
		Status: opts.status,
	}
}

// nulWriter wraps an io.Writer, replacing newline bytes with NUL bytes.
type nulWriter struct {
	w io.Writer
}

// Write replaces all newline bytes with NUL bytes before writing.
func (n *nulWriter) Write(p []byte) (int, error) {
	replaced := bytes.ReplaceAll(p, []byte{'\n'}, []byte{0})
	written, err := n.w.Write(replaced)
	if written == len(replaced) {
		return len(p), err
	}
	return written, err
}

// parseFlags parses GNU sha512sum-compatible flags from the argument list.
func parseFlags(args []string) (sha512sumOptions, []string) {
	var opts sha512sumOptions
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
				fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
				os.Exit(1)
			}
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// handleLongFlag processes a long-form flag and returns true if handled.
func handleLongFlag(arg string, opts *sha512sumOptions) bool {
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
	case "--strict":
		opts.strict = true
	case "--zero":
		opts.zero = true
	case "--version":
		fmt.Printf("%s (go-unix-utils) %s\n", programName, version)
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
func applyShortFlags(flags string, opts *sha512sumOptions) error {
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
		case 'z':
			opts.zero = true
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: sha512sum [OPTION]... [FILE]...
Print or check SHA512 (512-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary   read in binary mode
  -c, --check    read checksums from the FILEs and check them
      --tag      create a BSD-style checksum
  -t, --text     read in text mode (default)
  -z, --zero     end each output line with NUL, not newline
  -w, --warn     warn about improperly formatted checksum lines
      --quiet    don't print OK for each successfully verified file
      --status   don't output anything, status code shows success
      --strict   exit non-zero for improperly formatted checksum lines
      --help     display this help and exit
      --version  output version information and exit
`)
}
