// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/sha512sum: compute and check SHA-512 message digests.
// Implements srd033-sha512sum R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2,
// R4.1, R4.2, R4.3.
package main

import (
	"crypto/sha512"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	programName   = "sha512sum"
	algorithmName = "SHA512"
	// R1.3: SHA-512 produces 64-byte (128 hex character) digests.
	sha512DigestLen = 128
)

// usageText is the --help output printed to stdout.
const usageText = `Usage: sha512sum [OPTION]... [FILE]...
Print or check SHA512 (512-bit) checksums.

With no FILE, or when FILE is -, read standard input.

  -b, --binary   read in binary mode
  -c, --check    read checksums from the FILEs and check them
      --tag      create a BSD-style checksum
  -t, --text     read in text mode (default)

The following five options are useful only when verifying checksums:
      --quiet    don't print OK for each successfully verified file
      --status   don't output anything, status code shows success
      --strict   exit non-zero for improperly formatted checksum lines
  -w, --warn     warn about improperly formatted checksum lines

      --help     display this help and exit
      --version  output version information and exit
`

// versionText is the --version output printed to stdout.
const versionText = "sha512sum (go-unix-utils) 0.1.0\n"

// config holds parsed command-line options for sha512sum.
type config struct {
	binary  bool // -b, --binary
	text    bool // -t, --text
	tag     bool // --tag
	check   bool // -c, --check
	warn    bool // -w, --warn
	quiet   bool // --quiet
	status  bool // --status
	strict  bool // --strict
	help    bool // --help
	version bool // --version
	files   []string
}

// R1.1: main entry with SIGPIPE handler and flag parsing.
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

// run executes the sha512sum logic and returns the exit code.
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}

	// R1.3: configure HashConfig with SHA512 algorithm.
	hcfg := hashutil.HashConfig{
		Algorithm: algorithmName,
		NewHash:   sha512.New,
		DigestLen: sha512DigestLen,
	}

	if cfg.check {
		return runCheck(cfg, hcfg)
	}
	return hashutil.DigestFiles(cfg.files, hcfg, cfg.binary, cfg.tag, os.Stdout, os.Stderr)
}

// runCheck verifies checksums from each file argument.
// R2.1: reads checksum file, parses lines, recomputes and compares digests.
// R2.2: prints "FILENAME: OK" or "FILENAME: FAILED" per entry via hashutil.
// R2.3: --warn, --quiet, --status control verification output.
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
// R1.2: supports -b, -t, --tag, -c, -w, --quiet, --status, --strict, --help, --version.
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
		skip, err := parseFlag(&cfg, arg)
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

// validateArgs checks for invalid flag combinations.
func validateArgs(cfg config) error {
	if cfg.check && len(cfg.files) == 0 {
		return fmt.Errorf("--check requires a file argument")
	}
	return nil
}

// parseFlag dispatches to long or short flag parsing.
func parseFlag(cfg *config, arg string) (int, error) {
	if strings.HasPrefix(arg, "--") {
		return parseLongFlag(cfg, arg)
	}
	return parseShortFlags(cfg, arg[1:])
}

// parseLongFlag handles --name flags.
func parseLongFlag(cfg *config, arg string) (int, error) {
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

// parseShortFlags processes bundled short flags like -bw.
func parseShortFlags(cfg *config, flags string) (int, error) {
	for _, ch := range flags {
		switch ch {
		case 'b':
			cfg.binary = true
		case 't':
			cfg.text = true
		case 'c':
			cfg.check = true
		case 'w':
			cfg.warn = true
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}
