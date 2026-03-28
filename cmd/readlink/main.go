// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/readlink implements prd050-readlink R1.1–R1.6, R2.1–R2.2, R3.1–R3.2.
// It prints the target of a symbolic link or canonicalizes paths.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "readlink"

var errVersion = errors.New("version requested")

// canonMode represents the canonicalization mode.
type canonMode int

const (
	modeDefault  canonMode = iota // R1.1, R1.2: read symlink target
	modeCanon                     // R1.3: -f, all but last must exist
	modeExisting                  // R1.4: -e, all must exist
	modeMissing                   // R1.5: -m, nothing need exist
)

// config holds parsed command-line options.
type config struct {
	mode      canonMode
	noNewline bool // R1.6: -n/--no-newline
	zero      bool // -z/--zero: NUL delimiter
	verbose   bool // R2.2: -v/--verbose: report errors
}

// R1.1: Install SIGPIPE handler at startup.
func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run())
}

// run parses flags, validates arguments, and resolves each path.
func run() int {
	cfg, err := parseFlags()
	if err != nil {
		return handleParseError(err)
	}
	return processArgs(cfg)
}

// handleParseError handles flag parsing errors, --help, and --version.
func handleParseError(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		printUsage(os.Stdout)
		return 0
	}
	if errors.Is(err, errVersion) {
		printVersion()
		return 0
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
	return 1
}

// processArgs iterates over arguments and resolves each.
// R2.1: prints each result on a separate line.
// R3.1: exits 0 when all succeed, 1 when any fails.
func processArgs(cfg config) int {
	args := flag.Args()
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		return 1
	}
	exitCode := 0
	for i, arg := range args {
		if err := processArg(arg, cfg, i, len(args)); err != nil {
			reportError(arg, err, cfg)
			exitCode = 1
		}
	}
	return exitCode
}

// processArg resolves a single path and prints it to stdout.
func processArg(arg string, cfg config, _, total int) error {
	result, err := resolveArg(arg, cfg)
	if err != nil {
		return err
	}
	delim := delimiter(cfg, total)
	_, err = fmt.Fprintf(os.Stdout, "%s%s", result, delim)
	return err
}

// delimiter returns the appropriate line ending.
// R1.6: -n suppresses newline for a single operand.
// R2.2 (PRD): -n is ignored when multiple operands are given.
func delimiter(cfg config, total int) string {
	if cfg.zero {
		return "\x00"
	}
	if cfg.noNewline && total == 1 {
		return ""
	}
	return "\n"
}

// resolveArg dispatches to the appropriate resolution strategy.
func resolveArg(arg string, cfg config) (string, error) {
	switch cfg.mode {
	case modeCanon:
		return resolveCanon(arg)
	case modeExisting:
		return resolveExisting(arg)
	case modeMissing:
		return resolveMissing(arg)
	default:
		return readLink(arg)
	}
}

// readLink reads the immediate symlink target.
// R1.1: print target of symlink. R1.2: exit 1 if not a symlink.
func readLink(path string) (string, error) {
	return os.Readlink(path)
}

// resolveCanon resolves the canonical path; last component need not exist.
// R1.3: -f/--canonicalize.
func resolveCanon(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

// resolveExisting resolves the canonical path; all components must exist.
// R1.4: -e/--canonicalize-existing.
func resolveExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// resolveMissing resolves the canonical path; no component need exist.
// R1.5: -m/--canonicalize-missing.
func resolveMissing(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return resolved, nil
	}
	dir := filepath.Dir(abs)
	base := filepath.Base(abs)
	resolvedDir, err := resolveMissing(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolvedDir, base), nil
}

// parseFlags defines and parses all command-line flags.
func parseFlags() (config, error) {
	var cfg config
	var canonF, canonE, canonM bool
	var showHelp, showVersion bool

	flag.CommandLine = flag.NewFlagSet(programName, flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	// R1.3: -f / --canonicalize
	flag.BoolVar(&canonF, "f", false, "")
	flag.BoolVar(&canonF, "canonicalize", false, "")
	// R1.4: -e / --canonicalize-existing
	flag.BoolVar(&canonE, "e", false, "")
	flag.BoolVar(&canonE, "canonicalize-existing", false, "")
	// R1.5: -m / --canonicalize-missing
	flag.BoolVar(&canonM, "m", false, "")
	flag.BoolVar(&canonM, "canonicalize-missing", false, "")
	// R1.6: -n / --no-newline
	flag.BoolVar(&cfg.noNewline, "n", false, "")
	flag.BoolVar(&cfg.noNewline, "no-newline", false, "")
	// -z / --zero
	flag.BoolVar(&cfg.zero, "z", false, "")
	flag.BoolVar(&cfg.zero, "zero", false, "")
	// R2.2: -v / --verbose
	flag.BoolVar(&cfg.verbose, "v", false, "")
	flag.BoolVar(&cfg.verbose, "verbose", false, "")
	// R3.2: --help
	flag.BoolVar(&showHelp, "help", false, "")
	// R3.2: --version
	flag.BoolVar(&showVersion, "version", false, "")

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		return config{}, err
	}
	if showHelp {
		return config{}, flag.ErrHelp
	}
	if showVersion {
		return config{}, errVersion
	}
	cfg.mode = selectMode(canonF, canonE, canonM)
	return cfg, nil
}

// selectMode returns the canonicalization mode from flag booleans.
func selectMode(f, e, m bool) canonMode {
	switch {
	case m:
		return modeMissing
	case e:
		return modeExisting
	case f:
		return modeCanon
	default:
		return modeDefault
	}
}

// reportError conditionally prints an error based on mode and verbose flag.
// R2.2: -v/--verbose reports error messages for individual failures.
// Canon modes (-f, -e, -m) always report errors; default mode requires -v.
func reportError(path string, err error, cfg config) {
	if cfg.mode == modeDefault && !cfg.verbose {
		return
	}
	printError(path, err)
}

// printError writes a GNU-format error message to stderr.
// R3.2: error format matches GNU readlink output.
func printError(path string, err error) {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, path, pathErr.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, path, err)
}

// printUsage writes GNU-format usage information to the given writer.
// R3.2: --help prints usage to stdout and exits 0.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... FILE...\n", programName)
	fmt.Fprintln(w, "Print value of a symbolic link or canonical file name")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -f, --canonicalize            canonicalize by following every symlink in")
	fmt.Fprintln(w, "                                  every component of the given name recursively;")
	fmt.Fprintln(w, "                                  all but the last component must exist")
	fmt.Fprintln(w, "  -e, --canonicalize-existing    canonicalize by following every symlink in")
	fmt.Fprintln(w, "                                  every component of the given name recursively,")
	fmt.Fprintln(w, "                                  all components must exist")
	fmt.Fprintln(w, "  -m, --canonicalize-missing     canonicalize by following every symlink in")
	fmt.Fprintln(w, "                                  every component of the given name recursively,")
	fmt.Fprintln(w, "                                  without requirements on components existence")
	fmt.Fprintln(w, "  -n, --no-newline               do not output the trailing delimiter")
	fmt.Fprintln(w, "  -v, --verbose                  report error messages")
	fmt.Fprintln(w, "  -z, --zero                     end each output line with NUL, not newline")
	fmt.Fprintln(w, "      --help                     display this help and exit")
	fmt.Fprintln(w, "      --version                  output version information and exit")
}

// printVersion writes version information to stdout.
// R3.2: --version prints version and exits 0.
func printVersion() {
	fmt.Println("readlink (go-unix-utils) 1.0")
}
