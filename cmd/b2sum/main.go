// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd076-b2sum R1.1-R1.4, R2.1-R2.2: Compute and check BLAKE2b
// message digests using pkg/hashutil for shared formatting and verification.
// Supports --length for variable digest sizes, --tag for BSD-style output,
// and --check with --warn, --quiet, --status options.
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

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// defaultLengthBits is the default BLAKE2b digest length in bits.
const defaultLengthBits = 512

// b2sumOptions holds the parsed flag state for a b2sum invocation.
type b2sumOptions struct {
	binary bool // -b/--binary: asterisk before filename (R1.1)
	text   bool // -t/--text: default mode, two spaces (R1.1)
	tag    bool // --tag: BSD-style output (R1.3)
	check  bool // -c/--check: verify checksums (R2.1)
	warn   bool // -w/--warn: warn on malformed lines (R2.2)
	quiet  bool // --quiet: suppress OK lines (R2.2)
	status bool // --status: suppress all output (R2.2)
	strict bool // --strict: exit non-zero on malformed lines
	length int  // --length=N: digest length in bits (R3.3)
}

func main() {
	// R4.3: Install SIGPIPE handler per shared protocol.
	sys.InstallSIGPIPEHandler()

	opts, files := parseFlags(os.Args[1:])
	exitCode := run(opts, files)
	os.Exit(exitCode)
}

// buildConfig creates a HashConfig for BLAKE2b with the configured digest length.
// R1.3: algorithm name includes bit length when not the default 512.
func buildConfig(opts b2sumOptions) hashutil.HashConfig {
	byteLen := opts.length / 8
	algorithm := algorithmName(opts.length)
	return hashutil.HashConfig{
		Algorithm: algorithm,
		NewHash:   newBlake2bFactory(byteLen),
		DigestLen: byteLen * 2, // hex-encoded length
	}
}

// algorithmName returns the algorithm string for tag output.
// R1.3: "BLAKE2b" for 512-bit, "BLAKE2b-N" otherwise.
func algorithmName(bits int) string {
	if bits == defaultLengthBits {
		return "BLAKE2b"
	}
	return fmt.Sprintf("BLAKE2b-%d", bits)
}

// newBlake2bFactory returns a hash.Hash constructor for the given byte length.
func newBlake2bFactory(byteLen int) func() hash.Hash {
	return func() hash.Hash {
		h, _ := blake2b.New(byteLen, nil) // nil key; error only for invalid sizes
		return h
	}
}

// run executes the b2sum command with the given options and files.
// D2: delegates to hashutil.DigestFiles for digest mode,
// hashutil.VerifyChecksums for check mode.
func run(opts b2sumOptions, files []string) int {
	cfg := buildConfig(opts)
	if opts.check {
		return runCheck(opts, files, cfg)
	}
	return hashutil.DigestFiles(
		files, cfg, opts.binary, opts.tag, os.Stdout, os.Stderr,
	)
}

// runCheck verifies checksums from the given files.
// R2.1-R2.2: reads checksum file, prints OK/FAILED per entry.
func runCheck(opts b2sumOptions, files []string, cfg hashutil.HashConfig) int {
	if len(files) == 0 {
		files = []string{"-"}
	}
	checkOpts := buildCheckOptions(opts)
	exitCode := 0
	for _, f := range files {
		if !verifyOneFile(f, cfg, checkOpts) {
			exitCode = 1
		}
	}
	return exitCode
}

// verifyOneFile verifies checksums from a single file and returns true if all pass.
func verifyOneFile(f string, cfg hashutil.HashConfig, opts hashutil.CheckOptions) bool {
	ok, err := hashutil.VerifyChecksums(
		f, cfg, opts, os.Stdout, os.Stderr,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "b2sum: %v\n", err)
		return false
	}
	return ok
}

// buildCheckOptions constructs CheckOptions from b2sumOptions.
func buildCheckOptions(opts b2sumOptions) hashutil.CheckOptions {
	return hashutil.CheckOptions{
		Warn:   opts.warn || opts.strict,
		Quiet:  opts.quiet,
		Status: opts.status,
	}
}

// parseFlags parses GNU b2sum-compatible flags from the argument list.
// D5: uses manual parsing for both short and long flag forms.
func parseFlags(args []string) (b2sumOptions, []string) {
	var opts b2sumOptions
	opts.length = defaultLengthBits
	var files []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags {
			files = append(files, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if consumed := handleLongFlag(arg, args, i, &opts); consumed > 0 {
			i += consumed - 1
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg != "-" {
			if err := applyShortFlags(arg[1:], &opts); err != nil {
				fmt.Fprintf(os.Stderr, "b2sum: %s\n", err)
				os.Exit(1)
			}
			continue
		}
		files = append(files, arg)
	}
	return opts, files
}

// handleLongFlag processes a long-form flag and returns the number of args consumed (0 if not handled).
func handleLongFlag(arg string, args []string, idx int, opts *b2sumOptions) int {
	if !strings.HasPrefix(arg, "--") {
		return 0
	}
	if strings.HasPrefix(arg, "--length") {
		return parseLengthFlag(arg, args, idx, opts)
	}
	return handleSimpleLongFlag(arg, opts)
}

// handleSimpleLongFlag processes long flags that take no value.
func handleSimpleLongFlag(arg string, opts *b2sumOptions) int {
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
		fmt.Printf("b2sum (go-unix-utils) %s\n", version)
		os.Exit(0)
	case "--help":
		printHelp()
		os.Exit(0)
	default:
		return 0
	}
	return 1
}

// parseLengthFlag parses --length=N or --length N and validates the value.
// R3.3: N must be a positive multiple of 8 and at most 512.
func parseLengthFlag(arg string, args []string, idx int, opts *b2sumOptions) int {
	var valStr string
	consumed := 1
	if strings.Contains(arg, "=") {
		valStr = arg[strings.Index(arg, "=")+1:]
	} else if idx+1 < len(args) {
		valStr = args[idx+1]
		consumed = 2
	} else {
		fmt.Fprintf(os.Stderr, "b2sum: option '--length' requires an argument\n")
		os.Exit(1)
	}
	opts.length = validateLength(valStr)
	return consumed
}

// validateLength parses and validates the --length value.
// R3.3: must be a positive multiple of 8, at most 512.
func validateLength(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n%8 != 0 || n > defaultLengthBits {
		fmt.Fprintf(os.Stderr,
			"b2sum: invalid length: %q (must be a positive multiple of 8, at most %d)\n",
			s, defaultLengthBits)
		os.Exit(1)
	}
	return n
}

// applyShortFlags applies a sequence of short flag characters to opts.
func applyShortFlags(flags string, opts *b2sumOptions) error {
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
		case 'l':
			// -l requires a value; not supported in short form for b2sum
			return fmt.Errorf("invalid option -- '%c' (use --length=N)", c)
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: b2sum [OPTION]... [FILE]...
Print or check BLAKE2b (512-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary         read in binary mode
  -c, --check          read checksums from the FILEs and check them
      --length=N       digest length in bits; must not exceed the max for
                         the blake2 algorithm and must be a multiple of 8
      --tag            create a BSD-style checksum
  -t, --text           read in text mode (default)
  -w, --warn           warn about improperly formatted checksum lines
      --quiet          don't print OK for each successfully verified file
      --status         don't output anything, status code shows success
      --strict         exit non-zero for improperly formatted checksum lines
      --help           display this help and exit
      --version        output version information and exit
`)
}
