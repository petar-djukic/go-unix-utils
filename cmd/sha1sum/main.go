// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/sha1sum: compute and check SHA-1 message digests.
// Implements srd031-sha1sum R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3, R3.1, R3.2,
// R4.1, R4.2, R4.3.
package main

import (
	"crypto/sha1"
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/hashutil"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const (
	programName   = "sha1sum"
	algorithmName = "SHA1"
	// R1.3: SHA-1 produces 20-byte (40 hex character) digests.
	sha1DigestLen = 40
)

// config holds parsed command-line options for sha1sum.
type config struct {
	binary bool // -b, --binary
	text   bool // -t, --text
	tag    bool // --tag
	check  bool // -c, --check
	warn   bool // -w, --warn
	quiet  bool // --quiet
	status bool // --status
	files  []string
}

// R1.1: main entry with SIGPIPE handler and flag parsing.
// R1.4: InstallSIGPIPEHandler for graceful SIGPIPE exit.
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

// run executes the sha1sum logic and returns the exit code.
// R1.1: in digest mode, delegates to hashutil.DigestFiles.
func run(cfg config) int {
	// R1.3: configure HashConfig with SHA1 algorithm.
	hcfg := hashutil.HashConfig{
		Algorithm: algorithmName,
		NewHash:   sha1.New,
		DigestLen: sha1DigestLen,
	}

	if cfg.check {
		return runCheck(cfg, hcfg)
	}
	return hashutil.DigestFiles(cfg.files, hcfg, cfg.binary, cfg.tag, os.Stdout, os.Stderr)
}

// runCheck verifies checksums from each file argument.
func runCheck(cfg config, hcfg hashutil.HashConfig) int {
	opts := hashutil.CheckOptions{
		Warn:   cfg.warn,
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
// R1.2: supports -b, -t, --tag, -c, -w, --quiet, --status.
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
