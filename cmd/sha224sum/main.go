// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd074-sha224sum R1.1-R1.4, R2.1-R2.3, R3.1-R3.2, R4.1-R4.3:
// Compute and check SHA-224 message digests using pkg/hashutil for shared
// formatting and verification. Check mode supports --warn, --quiet, --status.
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

// sha224Config is the HashConfig for SHA-224 digests.
// D2: Algorithm="SHA224", NewHash=sha256.New224, DigestLen=56 (hex length).
var sha224Config = hashutil.HashConfig{
	Algorithm: "SHA224",
	NewHash:   sha256.New224,
	DigestLen: 56,
}

// sha224sumOptions holds the parsed flag state for a sha224sum invocation.
type sha224sumOptions struct {
	binary bool // -b/--binary: asterisk before filename (R3.1)
	text   bool // -t/--text: default mode, two spaces (R3.1)
	tag    bool // --tag: BSD-style output (R3.2)
	check  bool // -c/--check: verify checksums (R2.1)
	warn   bool // -w/--warn: warn on malformed lines (R2.3)
	quiet  bool // --quiet: suppress OK lines (R2.3)
	status bool // --status: suppress all output (R2.3)
	strict bool // --strict: exit non-zero on malformed lines (R2.3)
}

func main() {
	// R4.3: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, files := parseFlags(os.Args[1:])
	exitCode := run(opts, files)
	os.Exit(exitCode)
}

// run executes the sha224sum command with the given options and files.
// Delegates to hashutil.DigestFiles for digest mode,
// hashutil.VerifyChecksums for check mode.
func run(opts sha224sumOptions, files []string) int {
	if opts.check {
		return runCheck(opts, files)
	}
	return hashutil.DigestFiles(
		files, sha224Config, opts.binary, opts.tag, os.Stdout, os.Stderr,
	)
}

// runCheck verifies checksums from the given files.
// R2.1-R2.2: reads checksum file, prints OK/FAILED per entry.
// R2.3: --strict exits non-zero when malformed lines are detected.
func runCheck(opts sha224sumOptions, files []string) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	checkOpts := buildCheckOptions(opts)
	stderr := stderrWriter(opts.strict)
	exitCode := 0
	for _, f := range files {
		ok, err := hashutil.VerifyChecksums(
			f, sha224Config, checkOpts, os.Stdout, stderr,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sha224sum: %v\n", err)
			exitCode = 1
		} else if !ok {
			exitCode = 1
		}
	}
	if opts.strict && hasMalformed(stderr) {
		exitCode = 1
	}
	return exitCode
}

// stderrWriter returns an io.Writer for check-mode stderr output.
// When strict is true, returns a malformedDetector to track warnings.
func stderrWriter(strict bool) io.Writer {
	if strict {
		return &malformedDetector{w: os.Stderr}
	}
	return os.Stderr
}

// hasMalformed checks if the writer is a malformedDetector that saw
// the "improperly formatted" warning. Returns false for other writers.
func hasMalformed(w io.Writer) bool {
	if md, ok := w.(*malformedDetector); ok {
		return md.found
	}
	return false
}

// malformedDetector wraps an io.Writer and detects the "improperly
// formatted" warning emitted by hashutil.VerifyChecksums.
// R2.3: enables --strict to exit non-zero on malformed lines.
type malformedDetector struct {
	w     io.Writer
	found bool
}

// Write passes through to the underlying writer and checks for
// the malformed warning marker.
func (m *malformedDetector) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("improperly formatted")) {
		m.found = true
	}
	return m.w.Write(p)
}

// buildCheckOptions constructs CheckOptions from sha224sumOptions.
// R2.3: --strict implies --warn behavior.
func buildCheckOptions(opts sha224sumOptions) hashutil.CheckOptions {
	return hashutil.CheckOptions{
		Warn:   opts.warn || opts.strict,
		Quiet:  opts.quiet,
		Status: opts.status,
	}
}

// parseFlags parses GNU sha224sum-compatible flags from the argument list.
func parseFlags(args []string) (sha224sumOptions, []string) {
	var opts sha224sumOptions
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
				fmt.Fprintf(os.Stderr, "sha224sum: %s\n", err)
				os.Exit(1)
			}
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// handleLongFlag processes a long-form flag and returns true if handled.
func handleLongFlag(arg string, opts *sha224sumOptions) bool {
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
	case "--version":
		fmt.Printf("sha224sum (go-unix-utils) %s\n", version)
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
func applyShortFlags(flags string, opts *sha224sumOptions) error {
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
// R1.4: --help writes to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: sha224sum [OPTION]... [FILE]...
Print or check SHA224 (224-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary   read in binary mode
  -c, --check    read checksums from the FILEs and check them
      --tag      create a BSD-style checksum
  -t, --text     read in text mode (default)
  -w, --warn     warn about improperly formatted checksum lines
      --quiet    don't print OK for each successfully verified file
      --status   don't output anything, status code shows success
      --strict   exit non-zero for improperly formatted checksum lines
      --help     display this help and exit
      --version  output version information and exit
`)
}
