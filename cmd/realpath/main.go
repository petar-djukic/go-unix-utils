// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/realpath implements prd049-realpath R1.1–R1.5, R2.1–R2.3, R3.1–R3.3.
// It prints the resolved absolute pathname for each argument.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "realpath"

var errVersion = errors.New("version requested")

// config holds the parsed command-line options.
type config struct {
	canonMissing bool
	noSymlinks   bool
	relativeTo   string
	relativeBase string
}

// R1.4: Install SIGPIPE handler at startup.
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

// processArgs iterates over arguments and resolves each path.
// R3.1: exits 1 with usage error if no operands given.
// R3.3: prints errors for failing paths, still processes remaining ones.
func processArgs(cfg config) int {
	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		return 1
	}
	exitCode := 0
	for _, arg := range flag.Args() {
		if err := processArg(arg, cfg); err != nil {
			printError(arg, err)
			exitCode = 1
		}
	}
	return exitCode
}

// processArg resolves a single path and prints it to stdout.
func processArg(arg string, cfg config) error {
	resolved, err := resolve(arg, cfg)
	if err != nil {
		return err
	}
	fmt.Println(applyRelative(resolved, cfg))
	return nil
}

// parseFlags defines and parses all command-line flags.
func parseFlags() (config, error) {
	var cfg config
	var canonExisting, showHelp, showVersion bool

	flag.CommandLine = flag.NewFlagSet(programName, flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)

	// R1.3: -e / --canonicalize-existing (default behavior, accepted as no-op).
	flag.BoolVar(&canonExisting, "e", false, "")
	flag.BoolVar(&canonExisting, "canonicalize-existing", false, "")
	// R1.4: -m / --canonicalize-missing.
	flag.BoolVar(&cfg.canonMissing, "m", false, "")
	flag.BoolVar(&cfg.canonMissing, "canonicalize-missing", false, "")
	// R1.5: -s / --strip / --no-symlinks.
	flag.BoolVar(&cfg.noSymlinks, "s", false, "")
	flag.BoolVar(&cfg.noSymlinks, "strip", false, "")
	flag.BoolVar(&cfg.noSymlinks, "no-symlinks", false, "")
	// R2.1: --relative-to=DIR.
	flag.StringVar(&cfg.relativeTo, "relative-to", "", "")
	// R2.2: --relative-base=DIR.
	flag.StringVar(&cfg.relativeBase, "relative-base", "", "")
	// R3.1: --help.
	flag.BoolVar(&showHelp, "help", false, "")
	// R3.2: --version.
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
	// Suppress unused variable warning for canonExisting (accepted as no-op).
	_ = canonExisting
	return cfg, nil
}

// resolve dispatches to the appropriate resolution strategy.
func resolve(path string, cfg config) (string, error) {
	switch {
	case cfg.canonMissing:
		return resolveMissing(path)
	case cfg.noSymlinks:
		return resolveNoSymlinks(path)
	default:
		return resolveCanonical(path)
	}
}

// resolveCanonical resolves symlinks and requires all components to exist.
// R1.1, R1.2, R1.3: filepath.Abs + filepath.EvalSymlinks.
func resolveCanonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// resolveNoSymlinks cleans the path without resolving symlinks.
// R1.5: only clean . and .. and make absolute.
func resolveNoSymlinks(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

// resolveMissing resolves existing components and constructs the rest.
// R1.4: no component needs to exist.
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

// applyRelative adjusts the resolved path based on --relative-to and --relative-base.
// R2.1: --relative-to alone makes all output relative.
// R2.2: --relative-base alone prints relative only if path is under base.
// R2.3: both together applies --relative-to only if path is under --relative-base.
func applyRelative(resolved string, cfg config) string {
	if cfg.relativeBase == "" && cfg.relativeTo == "" {
		return resolved
	}
	if cfg.relativeBase == "" {
		return relPath(resolved, cfg.relativeTo, cfg)
	}
	absBase := resolveDir(cfg.relativeBase, cfg)
	if !isUnder(resolved, absBase) {
		return resolved
	}
	relDir := cfg.relativeBase
	if cfg.relativeTo != "" {
		relDir = cfg.relativeTo
	}
	return relPath(resolved, relDir, cfg)
}

// resolveDir resolves a directory path using the current mode, falling back to Abs.
func resolveDir(dir string, cfg config) string {
	resolved, err := resolve(dir, cfg)
	if err != nil {
		abs, _ := filepath.Abs(dir)
		return filepath.Clean(abs)
	}
	return resolved
}

// relPath computes the relative path from relDir to resolved.
func relPath(resolved, relDir string, cfg config) string {
	absRelDir := resolveDir(relDir, cfg)
	rel, err := filepath.Rel(absRelDir, resolved)
	if err != nil {
		return resolved
	}
	return rel
}

// isUnder checks whether path is equal to or a subdirectory of base.
func isUnder(path, base string) bool {
	if path == base {
		return true
	}
	return strings.HasPrefix(path, base+string(filepath.Separator))
}

// printUsage writes GNU-format usage information to the given writer.
// R3.1: --help prints usage to stdout and exits 0.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... FILE...\n", programName)
	fmt.Fprintln(w, "Print the resolved absolute file name;")
	fmt.Fprintln(w, "all but the last component must exist")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -e, --canonicalize-existing  all components of the path must exist")
	fmt.Fprintln(w, "  -m, --canonicalize-missing   no path components need exist or be a directory")
	fmt.Fprintln(w, "      --relative-to=DIR        print the resolved path relative to DIR")
	fmt.Fprintln(w, "      --relative-base=DIR      print absolute paths unless paths below DIR")
	fmt.Fprintln(w, "  -s, --strip, --no-symlinks   don't expand symlinks")
	fmt.Fprintln(w, "      --help                   display this help and exit")
	fmt.Fprintln(w, "      --version                output version information and exit")
}

// printVersion writes version information to stdout.
// R3.2: --version prints version and exits 0.
func printVersion() {
	fmt.Println("realpath (go-unix-utils) 1.0")
}

// printError writes a GNU-format error message to stderr.
// R3.3: GNU-compatible error messages for path resolution failures.
func printError(path string, err error) {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, path, pathErr.Err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s: %s\n", programName, path, err)
}
