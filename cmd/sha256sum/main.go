// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd032-sha256sum R1.1-R1.4, R2.1-R2.2: Compute and check SHA-256
// message digests using pkg/hashutil for shared formatting and verification.
// Supports --binary, --text, --tag, --check, --zero output modes.
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// sha256Config is the HashConfig for SHA-256 digests.
// D3: Algorithm="SHA256", NewHash=sha256.New, DigestLen=32.
var sha256Config = hashutil.HashConfig{
	Algorithm: "SHA256",
	NewHash:   sha256.New,
	DigestLen: 32,
}

// sha256sumOptions holds the parsed flag state for a sha256sum invocation.
type sha256sumOptions struct {
	binary bool // -b/--binary: asterisk before filename (R1.3)
	text   bool // -t/--text: default mode, two spaces (R1.3)
	tag    bool // --tag: BSD-style output (R2.1)
	check  bool // -c/--check: verify checksums
	warn   bool // -w/--warn: warn on malformed lines
	quiet  bool // --quiet: suppress OK lines
	status bool // --status: suppress all output
	strict bool // --strict: exit non-zero on malformed lines
	zero   bool // -z/--zero: NUL terminator instead of newline (R2.2)
}

func main() {
	// R4.3: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, files := parseFlags(os.Args[1:])
	exitCode := run(opts, files)
	os.Exit(exitCode)
}

// run executes the sha256sum command with the given options and files.
// D2: delegates to hashutil.DigestFiles for digest mode,
// hashutil.VerifyChecksums for check mode.
func run(opts sha256sumOptions, files []string) int {
	if opts.check {
		return runCheck(opts, files)
	}
	stdout := wrapStdout(opts.zero)
	return hashutil.DigestFiles(
		files, sha256Config, opts.binary, opts.tag, stdout, os.Stderr,
	)
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
// R2.1-R2.2: reads checksum file, prints OK/FAILED per entry.
func runCheck(opts sha256sumOptions, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	checkOpts := buildCheckOptions(opts)
	exitCode := 0
	for _, f := range files {
		ok, err := hashutil.VerifyChecksums(
			f, sha256Config, checkOpts, os.Stdout, os.Stderr,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sha256sum: %v\n", err)
			exitCode = 1
		} else if !ok {
			exitCode = 1
		}
	}
	return exitCode
}

// buildCheckOptions constructs CheckOptions from sha256sumOptions.
func buildCheckOptions(opts sha256sumOptions) hashutil.CheckOptions {
	return hashutil.CheckOptions{
		Warn:   opts.warn || opts.strict,
		Quiet:  opts.quiet,
		Status: opts.status,
	}
}

// nulWriter wraps an io.Writer, replacing newline bytes with NUL bytes.
// R2.2: --zero flag support.
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

// parseFlags parses GNU sha256sum-compatible flags from the argument list.
// D5: uses manual parsing for both short and long flag forms.
func parseFlags(args []string) (sha256sumOptions, []string) {
	var opts sha256sumOptions
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
				fmt.Fprintf(os.Stderr, "sha256sum: %s\n", err)
				os.Exit(1)
			}
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// handleLongFlag processes a long-form flag and returns true if handled.
func handleLongFlag(arg string, opts *sha256sumOptions) bool {
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
		fmt.Printf("sha256sum (go-unix-utils) %s\n", version)
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
func applyShortFlags(flags string, opts *sha256sumOptions) error {
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
	fmt.Print(`Usage: sha256sum [OPTION]... [FILE]...
Print or check SHA256 (256-bit) checksums.

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
