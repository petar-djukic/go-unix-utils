// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd083-truncate: Shrink or Extend File Size.
// Covers R1.1-R1.4 (size specification, relative prefixes, no-create, multiple files),
// R2.1-R2.2 (reference file, error exit), R3.1-R3.3 (exit codes, SIGPIPE).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sizeparse"
	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags.
var version = "dev"

// sizeOp represents the relative size operation prefix.
// R1.2: +, -, <, >, /, % prefixes.
type sizeOp int

const (
	opAbsolute  sizeOp = iota // no prefix: set to exact size
	opGrow                    // +: extend by SIZE
	opShrink                  // -: shrink by SIZE
	opAtMost                  // <: at most SIZE
	opAtLeast                 // >: at least SIZE
	opRoundDown               // /: round down to multiple of SIZE
	opRoundUp                 // %: round up to multiple of SIZE
)

// config holds parsed flag state.
type config struct {
	sizeStr   string // raw -s value
	refFile   string // -r value
	noCreate  bool   // -c flag
	files     []string
	op        sizeOp
	sizeBytes int64
}

func main() {
	// R3.3: SIGPIPE handler per project convention.
	sys.InstallSIGPIPEHandler()

	cfg, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}

	os.Exit(run(cfg))
}

// run executes the truncate operation on all target files.
// R1.3: applies the same size operation to each file.
func run(cfg config) int {
	baseSize, err := resolveBaseSize(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "truncate: %v\n", err)
		return 1
	}

	exitCode := 0
	for _, f := range cfg.files {
		if err := processFile(f, cfg, baseSize); err != nil {
			fmt.Fprintf(os.Stderr, "truncate: %v\n", err)
			exitCode = 1
		}
	}
	return exitCode
}

// resolveBaseSize determines the base size from -r or 0.
// R2.1: -r uses RFILE's size as the base.
func resolveBaseSize(cfg config) (int64, error) {
	if cfg.refFile == "" {
		return 0, nil
	}
	info, err := os.Stat(cfg.refFile)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot stat %q: %v", cfg.refFile, err,
		)
	}
	return info.Size(), nil
}

// processFile applies the truncate operation to a single file.
// R1.4: when -c is set, skip files that do not exist.
func processFile(path string, cfg config, baseSize int64) error {
	currentSize, err := getOrCreateFile(path, cfg.noCreate)
	if err != nil {
		return err
	}
	if currentSize < 0 {
		return nil // -c and file does not exist: skip
	}

	targetSize := max(computeTarget(cfg, currentSize, baseSize), 0)
	return os.Truncate(path, targetSize)
}

// getOrCreateFile returns the current file size, creating the file if
// needed. Returns -1 when -c is set and the file does not exist.
func getOrCreateFile(path string, noCreate bool) (int64, error) {
	info, err := os.Stat(path)
	if err == nil {
		return info.Size(), nil
	}
	if !os.IsNotExist(err) {
		return 0, fmt.Errorf(
			"cannot stat %q: %v", path, err,
		)
	}
	// R1.4/R2.1: file does not exist.
	if noCreate {
		return -1, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot create %q: %v", path, err,
		)
	}
	f.Close() // best-effort close of newly created empty file
	return 0, nil
}

// computeTarget applies the size operation to determine the final size.
// R1.2: relative prefix operations. R1.4/R2.1: reference base size.
func computeTarget(cfg config, currentSize, baseSize int64) int64 {
	sz := cfg.sizeBytes
	// R2.1: when -r is set and op is absolute, use baseSize directly.
	if cfg.refFile != "" && cfg.op == opAbsolute {
		return baseSize
	}
	effective := currentSize
	if cfg.refFile != "" {
		effective = baseSize
	}
	return applyOp(cfg.op, effective, sz)
}

// applyOp applies the size operation to the effective size.
func applyOp(op sizeOp, effective, sz int64) int64 {
	switch op {
	case opAbsolute:
		return sz
	case opGrow:
		return effective + sz
	case opShrink:
		return effective - sz
	case opAtMost:
		if effective > sz {
			return sz
		}
		return effective
	case opAtLeast:
		if effective < sz {
			return sz
		}
		return effective
	case opRoundDown:
		return roundDown(effective, sz)
	case opRoundUp:
		return roundUp(effective, sz)
	default:
		return sz
	}
}

// roundDown rounds n down to the nearest multiple of m.
func roundDown(n, m int64) int64 {
	if m <= 0 {
		return n
	}
	return (n / m) * m
}

// roundUp rounds n up to the nearest multiple of m.
func roundUp(n, m int64) int64 {
	if m <= 0 {
		return n
	}
	return ((n + m - 1) / m) * m
}

// parseSizeSpec parses a SIZE string with optional operator prefix.
// R1.1: plain bytes or with suffixes. R1.2: operator prefixes.
func parseSizeSpec(s string) (sizeOp, int64, error) {
	if s == "" {
		return opAbsolute, 0, fmt.Errorf("invalid number: %q", s)
	}
	op, rest := extractOp(s)
	n, err := sizeparse.Parse(rest)
	if err != nil {
		return opAbsolute, 0, fmt.Errorf("invalid number: %q", s)
	}
	return op, n, nil
}

// extractOp strips the operator prefix from a size string.
func extractOp(s string) (sizeOp, string) {
	if len(s) == 0 {
		return opAbsolute, s
	}
	switch s[0] {
	case '+':
		return opGrow, s[1:]
	case '-':
		return opShrink, s[1:]
	case '<':
		return opAtMost, s[1:]
	case '>':
		return opAtLeast, s[1:]
	case '/':
		return opRoundDown, s[1:]
	case '%':
		return opRoundUp, s[1:]
	default:
		return opAbsolute, s
	}
}

// parseArgs processes command-line arguments and returns config.
// Returns exit code -1 to continue, >= 0 for early exit.
func parseArgs(args []string) (config, int) {
	var cfg config
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			cfg.files = append(cfg.files, args[i+1:]...)
			break
		}
		exit := parseOneArg(args, &i, &cfg)
		if exit >= 0 {
			return config{}, exit
		}
	}
	return validateConfig(cfg)
}

// parseOneArg handles a single argument.
func parseOneArg(args []string, i *int, cfg *config) int {
	arg := args[*i]
	switch arg {
	case "--help":
		return printHelp()
	case "--version":
		return printVersion()
	case "-c", "--no-create":
		cfg.noCreate = true
		return -1
	case "-o", "--io-blocks":
		// io-blocks is a GNU extension; silently accept for compat.
		return -1
	default:
		return parseFlagWithValue(args, i, cfg, arg)
	}
}

// parseFlagWithValue handles flags that take values or positional args.
func parseFlagWithValue(
	args []string, i *int, cfg *config, arg string,
) int {
	switch {
	case arg == "-s" || arg == "--size":
		return consumeStringArg(args, i, &cfg.sizeStr, "size")
	case strings.HasPrefix(arg, "-s"):
		cfg.sizeStr = arg[2:]
		return -1
	case strings.HasPrefix(arg, "--size="):
		cfg.sizeStr = arg[7:]
		return -1
	case arg == "-r" || arg == "--reference":
		return consumeStringArg(args, i, &cfg.refFile, "reference")
	case strings.HasPrefix(arg, "-r"):
		cfg.refFile = arg[2:]
		return -1
	case strings.HasPrefix(arg, "--reference="):
		cfg.refFile = arg[12:]
		return -1
	default:
		return handleDefault(arg, cfg)
	}
}

// handleDefault handles positional arguments or unknown flags.
func handleDefault(arg string, cfg *config) int {
	if strings.HasPrefix(arg, "-") && len(arg) > 1 {
		fmt.Fprintf(os.Stderr,
			"truncate: unrecognized option '%s'\n", arg)
		return 1
	}
	cfg.files = append(cfg.files, arg)
	return -1
}

// consumeStringArg reads the next argument as a string value.
func consumeStringArg(
	args []string, i *int, target *string, name string,
) int {
	if *i+1 >= len(args) {
		fmt.Fprintf(os.Stderr,
			"truncate: option '--%s' requires an argument\n", name)
		return 1
	}
	*i++
	*target = args[*i]
	return -1
}

// validateConfig checks that required flags and operands are present.
func validateConfig(cfg config) (config, int) {
	if cfg.sizeStr == "" && cfg.refFile == "" {
		fmt.Fprintln(os.Stderr,
			"truncate: you must specify either '--size' or '--reference'")
		return config{}, 1
	}
	if len(cfg.files) == 0 {
		fmt.Fprintln(os.Stderr,
			"truncate: missing file operand")
		return config{}, 1
	}
	if cfg.sizeStr != "" {
		op, sz, err := parseSizeSpec(cfg.sizeStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "truncate: %v\n", err)
			return config{}, 1
		}
		cfg.op = op
		cfg.sizeBytes = sz
	}
	return cfg, -1
}

// printHelp writes usage information and returns exit code 0.
func printHelp() int {
	fmt.Fprint(os.Stdout, `Usage: truncate OPTION... FILE...
Shrink or extend the size of each FILE to the specified size.

A FILE argument that does not exist is created.

If a FILE is larger than the specified size, the extra data is lost.
If a FILE is shorter, it is extended and the sparse extended part (hole)
reads as zero bytes.

  -c, --no-create        do not create any files
  -o, --io-blocks        treat SIZE as number of IO blocks instead of bytes
  -r, --reference=RFILE  base size on RFILE
  -s, --size=SIZE        set or adjust the file size by SIZE bytes
      --help     display this help and exit
      --version  output version information and exit

SIZE may be (or may be an integer optionally followed by) one of following:
KB 1000, K 1024, MB 1000*1000, M 1024*1024, and so on for G, T, P, E, Z, Y.

SIZE may also be prefixed by one of the following modifying characters:
'+' extend by, '-' reduce by, '<' at most, '>' at least,
'/' round down to multiple of, '%' round up to multiple of.
`)
	return 0
}

// printVersion writes version information and returns exit code 0.
func printVersion() int {
	fmt.Fprintf(os.Stdout, "truncate (go-unix-utils) %s\n", version)
	return 0
}
