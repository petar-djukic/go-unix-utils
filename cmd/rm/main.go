// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd058-rm: Remove files or directories.
// R1.1 (basic file removal), R1.2 (refuse directories without -r),
// R1.3 (refuse . and ..), R1.4 (continue on error),
// R2.1 (recursive removal), R2.2 (force mode).
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// config holds the parsed command-line flags for rm.
type config struct {
	force     bool
	recursive bool
	dir       bool
	verbose   bool
	// TODO: -i (always prompt) parsed but not implemented (prd058-rm R3.1).
	interactive bool
	// TODO: -I (once prompt) parsed but not implemented (prd058-rm R3.2).
	interactiveOnce bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	cfg, args, exitCode := parseArgs(os.Args[1:])
	if exitCode >= 0 {
		os.Exit(exitCode)
	}
	os.Exit(run(cfg, args))
}

// run executes the rm operation and returns the exit code.
// R4.1: exit 0 on success. R4.2: exit 1 on any error.
func run(cfg config, args []string) int {
	if len(args) == 0 {
		if cfg.force {
			return 0
		}
		printErr("missing operand")
		return 1
	}
	exitCode := 0
	for _, arg := range args {
		if removeOne(cfg, arg) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// removeOne removes a single path and returns 0 on success, 1 on error.
// R1.3: refuse . and .. entries.
// R1.4: report errors and continue.
func removeOne(cfg config, path string) int {
	if isDotOrDotDot(path) {
		printErr(
			"refusing to remove '.' or '..' directory: skipping '%s'",
			path)
		return 1
	}
	info, err := os.Lstat(path)
	if err != nil {
		return handleStatError(cfg, path, err)
	}
	if info.IsDir() {
		return handleDir(cfg, path)
	}
	return removeFile(cfg, path)
}

// handleStatError handles os.Lstat errors.
// R2.2: -f ignores nonexistent files and exits 0.
func handleStatError(cfg config, path string, err error) int {
	if os.IsNotExist(err) && cfg.force {
		return 0
	}
	printErr("cannot remove '%s': %v", path, unwrapErr(err))
	return 1
}

// handleDir routes directory removal based on flags.
// R1.2: without -r, refuse to remove directories.
// R2.1: -r removes directories recursively.
// R2.4: -d removes empty directories.
func handleDir(cfg config, path string) int {
	if cfg.recursive {
		return removeRecursive(cfg, path)
	}
	if cfg.dir {
		return removeSingleDir(cfg, path)
	}
	printErr("cannot remove '%s': Is a directory", path)
	return 1
}

// removeFile removes a single non-directory file.
// R1.1: remove file using os.Remove (calls unlink(2)).
func removeFile(cfg config, path string) int {
	if err := os.Remove(path); err != nil {
		printErr("cannot remove '%s': %v", path, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerbose(path)
	}
	return 0
}

// removeSingleDir removes a single empty directory.
// R2.4: -d removes empty directories.
func removeSingleDir(cfg config, path string) int {
	if err := os.Remove(path); err != nil {
		printErr("cannot remove '%s': %v", path, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerboseDir(path)
	}
	return 0
}

// removeRecursive removes a directory tree depth-first.
// R2.1: -r removes directories and contents recursively.
func removeRecursive(cfg config, path string) int {
	entries, err := os.ReadDir(path)
	if err != nil {
		printErr("cannot remove '%s': %v", path, unwrapErr(err))
		return 1
	}
	exitCode := removeEntries(cfg, path, entries)
	if removeEmptyDir(cfg, path) != 0 {
		exitCode = 1
	}
	return exitCode
}

// removeEntries removes all entries within a directory.
// R1.4: continues on error.
func removeEntries(
	cfg config, parent string, entries []os.DirEntry,
) int {
	exitCode := 0
	for _, entry := range entries {
		child := filepath.Join(parent, entry.Name())
		if removeOne(cfg, child) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// removeEmptyDir removes a now-empty directory after recursive removal.
func removeEmptyDir(cfg config, path string) int {
	if err := os.Remove(path); err != nil {
		printErr("cannot remove '%s': %v", path, unwrapErr(err))
		return 1
	}
	if cfg.verbose {
		printVerboseDir(path)
	}
	return 0
}

// isDotOrDotDot reports whether the path ends with "." or "..".
// R1.3: prevent accidental directory tree destruction.
func isDotOrDotDot(path string) bool {
	base := filepath.Base(path)
	return base == "." || base == ".."
}

// printVerbose prints the removal message for a file.
// R3.3: format matches GNU rm: "removed 'FILE'".
func printVerbose(path string) {
	fmt.Fprintf(os.Stdout, "removed '%s'\n", path)
}

// printVerboseDir prints the removal message for a directory.
func printVerboseDir(path string) {
	fmt.Fprintf(os.Stdout, "removed directory '%s'\n", path)
}

// printErr prints a formatted error to stderr in GNU rm format.
func printErr(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "rm: "+format+"\n", args...)
}

// unwrapErr extracts the inner error from os.PathError.
func unwrapErr(err error) error {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// parseArgs processes command-line flags and returns configuration.
// exit is -1 when processing should continue; >= 0 for early exit.
func parseArgs(
	args []string,
) (cfg config, operands []string, exit int) {
	exit = -1
	for i := 0; i < len(args); i++ {
		if args[i] == "--" {
			operands = append(operands, args[i+1:]...)
			return
		}
		exit = parseOneArg(args[i], &cfg, &operands)
		if exit >= 0 {
			return config{}, nil, exit
		}
	}
	return
}

// parseOneArg handles a single argument token.
func parseOneArg(
	arg string, cfg *config, operands *[]string,
) int {
	switch {
	case arg == "--help":
		return printHelp()
	case arg == "--version":
		return printVersion()
	case isLongFlag(arg):
		return parseLongFlag(arg, cfg)
	case strings.HasPrefix(arg, "-") && len(arg) > 1:
		return parseShortFlags(arg[1:], cfg)
	default:
		*operands = append(*operands, arg)
	}
	return -1
}

// isLongFlag returns true for --prefixed flags.
func isLongFlag(arg string) bool {
	return strings.HasPrefix(arg, "--") && len(arg) > 2
}

// parseLongFlag handles --option flags.
func parseLongFlag(arg string, cfg *config) int {
	switch arg {
	case "--force":
		cfg.force = true
	case "--recursive":
		cfg.recursive = true
	case "--dir":
		cfg.dir = true
	case "--verbose":
		cfg.verbose = true
	default:
		fmt.Fprintf(os.Stderr, "rm: unrecognized option '%s'\n", arg)
		return 1
	}
	return -1
}

// parseShortFlags processes clustered short flags like -rf.
func parseShortFlags(flags string, cfg *config) int {
	for j := 0; j < len(flags); j++ {
		if exit := applyShortFlag(flags[j], cfg); exit >= 0 {
			return exit
		}
	}
	return -1
}

// applyShortFlag applies a single short flag character.
func applyShortFlag(ch byte, cfg *config) int {
	switch ch {
	case 'f':
		cfg.force = true
	case 'r', 'R':
		cfg.recursive = true
	case 'd':
		cfg.dir = true
	case 'v':
		cfg.verbose = true
	case 'i':
		cfg.interactive = true
	case 'I':
		cfg.interactiveOnce = true
	default:
		fmt.Fprintf(os.Stderr,
			"rm: invalid option -- '%c'\n", ch)
		return 1
	}
	return -1
}

// printHelp writes usage information to stdout and returns exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: rm [OPTION]... [FILE]...
Remove (unlink) the FILE(s).

  -f, --force           ignore nonexistent files and arguments, never prompt
  -i                    prompt before every removal
  -I                    prompt once before removing more than three files, or
                          when removing recursively
  -r, -R, --recursive   remove directories and their contents recursively
  -d, --dir             remove empty directories
  -v, --verbose         explain what is being done
      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information and returns exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout,
		"rm (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
