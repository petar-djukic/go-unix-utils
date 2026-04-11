// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/mkfifo: create FIFOs (named pipes).
// Implements srd092 R1.1, R1.2, R1.3, R1.4, R2.1, R2.2, R2.3.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "mkfifo"

// usageText is the --help output printed to stdout.
const usageText = `Usage: mkfifo [OPTION]... NAME...
Create named pipes (FIFOs) with the given NAMEs.

Mandatory arguments to long options are mandatory for short options too.
  -m, --mode=MODE   set file permission bits to MODE, not a=rw - umask
      --help        display this help and exit
      --version     output version information and exit
`

// versionText is the --version output printed to stdout.
const versionText = "mkfifo (go-unix-utils) 0.1.0\n"

// defaultFIFOMode is the base mode for new FIFOs before umask.
// R1.3: 0666 modified by umask when -m is not given.
const defaultFIFOMode = os.FileMode(0o666)

// config holds parsed command-line options for mkfifo.
type config struct {
	mode    string // -m, --mode=MODE
	help    bool   // --help
	version bool   // --version
	names   []string
}

// R1.1, R2.3: main entry with SIGPIPE handler and flag parsing.
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

// run executes the mkfifo logic and returns the exit code.
// R1.2: processes each NAME argument independently.
// R1.4: prints error and continues on failure.
func run(cfg config) int {
	if cfg.help {
		fmt.Fprint(os.Stdout, usageText)
		return 0
	}
	if cfg.version {
		fmt.Fprint(os.Stdout, versionText)
		return 0
	}

	// R4: at least one operand required.
	if len(cfg.names) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		return 1
	}

	// R1.3: validate mode upfront before creating any FIFOs.
	if cfg.mode != "" {
		if _, err := parseMode(cfg.mode); err != nil {
			fmt.Fprintf(os.Stderr, "%s: invalid mode '%s'\n", programName, cfg.mode)
			return 1
		}
	}

	exitCode := 0
	for _, name := range cfg.names {
		if err := createFIFO(cfg, name); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// createFIFO creates a single FIFO at the given path.
// TODO: R1.1 — implement actual FIFO creation using syscall.Mkfifo
// or unix.Mkfifo. Currently stubbed pending full implementation.
func createFIFO(cfg config, path string) error {
	mode := defaultFIFOMode
	if cfg.mode != "" {
		parsed, err := parseMode(cfg.mode)
		if err != nil {
			return fmt.Errorf("invalid mode '%s'", cfg.mode)
		}
		mode = parsed
	}
	// TODO: Call syscall.Mkfifo(path, uint32(mode)) to create the FIFO.
	_ = mode
	_ = path
	return nil
}

// parseMode parses a mode string as either octal or symbolic.
// R1.3: supports both octal (0644) and symbolic (a=rw) forms.
func parseMode(mode string) (os.FileMode, error) {
	if len(mode) > 0 && mode[0] >= '0' && mode[0] <= '9' {
		return parseOctalMode(mode)
	}
	return parseSymbolicMode(mode, defaultFIFOMode)
}

// parseOctalMode parses an octal permission string like "0644" or "644".
func parseOctalMode(mode string) (os.FileMode, error) {
	val, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid mode '%s'", mode)
	}
	return os.FileMode(val), nil
}

// parseSymbolicMode parses a symbolic mode string like "a=rw"
// and applies it to the base mode.
func parseSymbolicMode(mode string, base os.FileMode) (os.FileMode, error) {
	result := base
	for _, clause := range strings.Split(mode, ",") {
		var err error
		result, err = applySymbolicClause(clause, result)
		if err != nil {
			return 0, err
		}
	}
	return result, nil
}

// applySymbolicClause applies one clause like "a=rw" or "go-w".
func applySymbolicClause(clause string, perm os.FileMode) (os.FileMode, error) {
	if clause == "" {
		return 0, fmt.Errorf("invalid mode clause")
	}
	who, rest := parseWho(clause)
	if len(rest) == 0 {
		return 0, fmt.Errorf("invalid mode clause '%s'", clause)
	}
	return applyPermOps(who, rest, perm)
}

// parseWho extracts the ugoa prefix from a symbolic clause, returning
// a bitmask and the remaining string after the who characters.
func parseWho(clause string) (uint, string) {
	var who uint
	i := 0
	for i < len(clause) {
		switch clause[i] {
		case 'u':
			who |= 0o700
		case 'g':
			who |= 0o070
		case 'o':
			who |= 0o007
		case 'a':
			who |= 0o777
		default:
			if who == 0 {
				who = 0o777
			}
			return who, clause[i:]
		}
		i++
	}
	if who == 0 {
		who = 0o777
	}
	return who, clause[i:]
}

// applyPermOps processes operator+permissions pairs in a clause.
func applyPermOps(who uint, rest string, perm os.FileMode) (os.FileMode, error) {
	for len(rest) > 0 {
		if rest[0] != '+' && rest[0] != '-' && rest[0] != '=' {
			return 0, fmt.Errorf("invalid operator '%c'", rest[0])
		}
		op := rest[0]
		rest = rest[1:]
		var bits os.FileMode
		rest, bits = parsePermBits(rest)
		perm = applyOp(perm, op, who, bits)
	}
	return perm, nil
}

// parsePermBits reads rwxXst characters and returns remaining string and bits.
func parsePermBits(s string) (string, os.FileMode) {
	var bits os.FileMode
	i := 0
	for i < len(s) {
		switch s[i] {
		case 'r':
			bits |= 0o444
		case 'w':
			bits |= 0o222
		case 'x', 'X':
			bits |= 0o111
		case 's':
			bits |= os.ModeSetuid | os.ModeSetgid
		case 't':
			bits |= os.ModeSticky
		default:
			return s[i:], bits
		}
		i++
	}
	return s[i:], bits
}

// applyOp applies a single +, -, or = operation with who mask.
func applyOp(perm os.FileMode, op byte, who uint, bits os.FileMode) os.FileMode {
	masked := (bits & os.FileMode(who)) |
		(bits & (os.ModeSetuid | os.ModeSetgid | os.ModeSticky))
	switch op {
	case '+':
		perm |= masked
	case '-':
		perm &^= masked
	case '=':
		perm = (perm &^ os.FileMode(who)) | masked
	}
	return perm
}

// parseArgs parses command-line arguments into config.
func parseArgs(args []string) (config, error) {
	cfg := config{}
	flagsDone := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || (!strings.HasPrefix(arg, "-") || arg == "-") {
			cfg.names = append(cfg.names, arg)
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
	return cfg, nil
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

	if strings.HasPrefix(arg, "--mode=") {
		cfg.mode = arg[len("--mode="):]
		return 0, nil
	}

	switch arg {
	case "--mode":
		if idx+1 >= len(args) {
			return 0, fmt.Errorf("option '--mode' requires an argument")
		}
		cfg.mode = args[idx+1]
		return 1, nil
	case "--help":
		cfg.help = true
	case "--version":
		cfg.version = true
	default:
		return 0, fmt.Errorf("unrecognized option '%s'", arg)
	}
	return 0, nil
}

// parseShortFlags processes bundled short flags like -m0644.
func parseShortFlags(cfg *config, args []string, idx int) (int, error) {
	flags := args[idx][1:]
	for i, ch := range flags {
		switch ch {
		case 'm':
			rest := flags[i+1:]
			if len(rest) > 0 {
				cfg.mode = rest
				return 0, nil
			}
			if idx+1 >= len(args) {
				return 0, fmt.Errorf("option requires an argument -- 'm'")
			}
			cfg.mode = args[idx+1]
			return 1, nil
		default:
			return 0, fmt.Errorf("invalid option -- '%c'", ch)
		}
	}
	return 0, nil
}
