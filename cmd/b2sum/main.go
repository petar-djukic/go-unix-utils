// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/b2sum: compute and check BLAKE2b message digests.
// Implements srd076-b2sum R1.1-R1.4, R2.1-R2.3, R3.1.
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
	programName = "b2sum"
	// R1.1: BLAKE2b-512 produces 64-byte (128 hex character) digests by default.
	defaultDigestBytes = 64
	defaultDigestHex   = 128
	defaultDigestBits  = 512
)

// usageText is the --help output printed to stdout.
const usageText = `Usage: b2sum [OPTION]... [FILE]...
Print or check BLAKE2 (512-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary         read in binary mode
  -c, --check          read checksums from the FILEs and check them
  -l, --length=BITS    digest length in bits; must not exceed the max for
                         the blake2 algorithm and must be a multiple of 8
      --tag            create a BSD-style checksum
  -t, --text           read in text mode (default)

The following five options are useful only when verifying checksums:
      --quiet          don't print OK for each successfully verified file
      --status         don't output anything, status code shows success
      --strict         exit non-zero for improperly formatted checksum lines
  -w, --warn           warn about improperly formatted checksum lines

      --help           display this help and exit
      --version        output version information and exit
`

// versionText is the --version output printed to stdout.
const versionText = "b2sum (go-unix-utils) 0.1.0\n"

// config holds parsed command-line options for b2sum.
type config struct {
	binary  bool // -b, --binary
	text    bool // -t, --text
	tag     bool // --tag
	check   bool // -c, --check
	length  int  // -l, --length (bits); 0 means default 512
	warn    bool // -w, --warn
	quiet   bool // --quiet
	status  bool // --status
	strict  bool // --strict
	help    bool // --help
	version bool // --version
	files   []string
}

// R1.1: main entry with SIGPIPE handler and flag parsing.
// R4.3: InstallSIGPIPEHandler for graceful SIGPIPE exit.
func main() {
	sys.InstallSIGPIPEHandler()

	cfg, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		os.Exit(1)
	}

	exitCode := run(cfg)
	os.Exit(exitCode)
}

// run executes the b2sum logic and returns the exit code.
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}

	hcfg, err := buildHashConfig(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
		return 1
	}

	if cfg.check {
		return runCheck(cfg, hcfg)
	}
	return hashutil.DigestFiles(cfg.files, hcfg, cfg.binary, cfg.tag, os.Stdout, os.Stderr)
}

// buildHashConfig constructs a HashConfig for the configured digest length.
// R1.2: default BLAKE2b-512; R3.3: --length sets variable digest size.
func buildHashConfig(cfg config) (hashutil.HashConfig, error) {
	bits := defaultDigestBits
	if cfg.length > 0 {
		bits = cfg.length
	}

	digestBytes := bits / 8
	newHash, err := newBlake2bFactory(digestBytes)
	if err != nil {
		return hashutil.HashConfig{}, err
	}

	algo := algorithmName(bits)
	return hashutil.HashConfig{
		Algorithm: algo,
		NewHash:   newHash,
		DigestLen: digestBytes * 2, // hex characters
	}, nil
}

// newBlake2bFactory returns a hash.Hash factory for the given byte size.
func newBlake2bFactory(size int) (func() hash.Hash, error) {
	// Validate by creating one instance.
	_, err := blake2b.New(size, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid digest length: %w", err)
	}
	return func() hash.Hash {
		h, _ := blake2b.New(size, nil) // validated above; cannot fail
		return h
	}, nil
}

// algorithmName returns "BLAKE2b" for default 512-bit or "BLAKE2b-N" for
// non-default lengths, matching GNU b2sum --tag output.
// R1.3: tag name includes bit length when --length is specified.
func algorithmName(bits int) string {
	if bits == defaultDigestBits {
		return "BLAKE2b"
	}
	return fmt.Sprintf("BLAKE2b-%d", bits)
}

// runCheck verifies checksums from each file argument.
func runCheck(cfg config, hcfg hashutil.HashConfig) int {
	opts := hashutil.CheckOptions{
		Warn:   cfg.warn || cfg.strict,
		Quiet:  cfg.quiet,
		Status: cfg.status,
	}
	allOK := true
	for _, f := range cfg.files {
		ok, err := hashutil.VerifyChecksums(f, hcfg, opts, os.Stdout, os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
			allOK = false
			continue
		}
		if !ok {
			allOK = false
		}
	}
	if !allOK {
		return 1
	}
	return 0
}

// parseArgs parses command-line arguments into config.
func parseArgs(args []string) (config, error) {
	cfg := config{}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (!strings.HasPrefix(arg, "-") || arg == "-") {
			cfg.files = append(cfg.files, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		skip, err := parseFlag(&cfg, args, i)
		if err != nil {
			return config{}, err
		}
		i += skip
	}
	if cfg.help || cfg.version {
		return cfg, nil
	}
	return cfg, validateArgs(cfg)
}

// validateArgs checks for invalid flag combinations and length value.
// R2.3: --binary, --text, --tag are invalid with --check.
func validateArgs(cfg config) error {
	if cfg.check && len(cfg.files) == 0 {
		return fmt.Errorf("--check requires a file argument")
	}
	if err := validateCheckCombos(cfg); err != nil {
		return err
	}
	if cfg.length > 0 {
		if cfg.length%8 != 0 {
			return fmt.Errorf("invalid length: %d is not a multiple of 8", cfg.length)
		}
		if cfg.length > defaultDigestBits {
			return fmt.Errorf("invalid length: %d exceeds maximum of %d", cfg.length, defaultDigestBits)
		}
	}
	return nil
}

// validateCheckCombos rejects --binary, --text, and --tag when --check is set.
// R2.3: these format flags are meaningless in verification mode.
func validateCheckCombos(cfg config) error {
	if !cfg.check {
		return nil
	}
	if cfg.binary {
		return fmt.Errorf("the --binary and --check options are mutually exclusive")
	}
	if cfg.text {
		return fmt.Errorf("the --text and --check options are mutually exclusive")
	}
	if cfg.tag {
		return fmt.Errorf("the --tag and --check options are mutually exclusive")
	}
	return nil
}

// parseFlag dispatches to long or short flag parsing.
func parseFlag(cfg *config, args []string, idx int) (int, error) {
	arg := args[idx]
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(cfg, args, idx)
	}
	return parseShortFlags(cfg, args, idx)
}

// parseLongFlag handles --name and --name=value flags.
func parseLongFlag(cfg *config, args []string, idx int) (int, error) {
	arg := args[idx]

	// Handle --length=N or --length N
	if arg == "--length" || strings.HasPrefix(arg, "--length=") {
		return parseLengthFlag(cfg, arg, args, idx)
	}

	switch arg {
	case "--binary":
		cfg.binary = true
	case "--text":
		cfg.text = true
	case "--tag":
		cfg.tag = true
	case "--check":
		cfg.check = true
	case "--warn":
		cfg.warn = true
	case "--quiet":
		cfg.quiet = true
	case "--status":
		cfg.status = true
	case "--strict":
		cfg.strict = true
	case "--help":
		cfg.help = true
	case "--version":
		cfg.version = true
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 0, nil
}

// parseLengthFlag parses --length=N or --length N.
func parseLengthFlag(cfg *config, arg string, args []string, idx int) (int, error) {
	var valStr string
	var skip int
	if strings.Contains(arg, "=") {
		valStr = arg[strings.Index(arg, "=")+1:]
	} else {
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--length' requires an argument")
		}
		valStr = args[idx+1]
		skip = 1
	}
	n, err := strconv.Atoi(valStr)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid length value: '%s'", valStr)
	}
	cfg.length = n
	return skip, nil
}

// parseShortFlags processes bundled short flags like -bw.
// -l requires a value: -l256 or -l 256.
func parseShortFlags(cfg *config, args []string, idx int) (int, error) {
	flags := args[idx][1:]
	for i := 0; i < len(flags); i++ {
		ch := flags[i]
		switch ch {
		case 'b':
			cfg.binary = true
		case 't':
			cfg.text = true
		case 'c':
			cfg.check = true
		case 'w':
			cfg.warn = true
		case 'l':
			return parseShortLength(cfg, flags[i+1:], args, idx)
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}

// parseShortLength handles -lN or -l N after the 'l' character.
func parseShortLength(cfg *config, rest string, args []string, idx int) (int, error) {
	var valStr string
	var skip int
	if len(rest) > 0 {
		valStr = rest
	} else {
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option requires an argument -- 'l'")
		}
		valStr = args[idx+1]
		skip = 1
	}
	n, err := strconv.Atoi(valStr)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid length value: '%s'", valStr)
	}
	cfg.length = n
	return skip, nil
}
