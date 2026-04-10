// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/rm: remove files or directories.
// Implements srd058 R1.1-R1.4, R2.1-R2.4.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "rm"

// options holds parsed command-line flags for rm.
type options struct {
	recursive bool // -r, -R, --recursive
	force     bool // -f, --force
	dir       bool // -d, --dir
	verbose   bool // -v, --verbose
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, args := parseArgs(os.Args[1:])
	os.Exit(run(opts, args))
}

// run validates arguments and dispatches removal.
// R2.2: with -f and no arguments, exit 0 silently.
func run(opts options, args []string) int {
	if len(args) == 0 {
		if opts.force {
			return 0
		}
		printMissingOperand()
		return 1
	}
	return removeArgs(opts, args)
}

// removeArgs removes each argument, continuing on errors.
// R1.4: errors are printed to stderr; processing continues.
func removeArgs(opts options, args []string) int {
	exitCode := 0
	for _, arg := range args {
		if err := removeOne(opts, arg); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			exitCode = 1
		}
	}
	return exitCode
}

// removeOne removes a single path.
// R1.3: refuses . and .. before any filesystem access.
func removeOne(opts options, path string) error {
	if isDotOrDotDot(path) {
		return fmt.Errorf(
			"refusing to remove '.' or '..' directory: skipping '%s'",
			path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		if opts.force && os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	if info.IsDir() {
		return handleDir(opts, path)
	}
	return removeFile(opts, path)
}

// isDotOrDotDot returns true if path's base is "." or "..".
// R1.3: prevents accidental directory tree destruction.
func isDotOrDotDot(path string) bool {
	base := filepath.Base(path)
	return base == "." || base == ".."
}

// handleDir handles directory removal based on flags.
// R1.2: without -r or -d, refuses to remove directories.
// R2.1: -r removes directories and their contents recursively.
// R2.4: -d removes empty directories only.
func handleDir(opts options, path string) error {
	if opts.recursive {
		return removeRecursive(opts, path)
	}
	if opts.dir {
		return removeDirEntry(opts, path)
	}
	return fmt.Errorf("cannot remove '%s': Is a directory", path)
}

// removeFile removes a regular file or symlink.
// R1.1: uses os.Remove (unlink(2)) for file removal.
func removeFile(opts options, path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	if opts.verbose {
		printRemoved(path)
	}
	return nil
}

// removeRecursive recursively removes a directory tree.
// R2.1: -r removes directories and their contents.
// R2.3: combined with -f, silently removes without prompting.
func removeRecursive(opts options, path string) error {
	if opts.verbose {
		return removeTreeVerbose(path)
	}
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	return nil
}

// removeTreeVerbose removes a directory tree, printing each entry.
func removeTreeVerbose(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		if err := removeChildVerbose(child, entry); err != nil {
			return err
		}
	}
	return removeDirAndPrint(path)
}

// removeChildVerbose removes a single child entry with verbose output.
func removeChildVerbose(child string, entry os.DirEntry) error {
	if entry.IsDir() {
		return removeTreeVerbose(child)
	}
	if err := os.Remove(child); err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			child, sysErrMsg(err))
	}
	printRemoved(child)
	return nil
}

// removeDirAndPrint removes a directory and prints verbose output.
func removeDirAndPrint(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	printRemovedDir(path)
	return nil
}

// removeDirEntry removes an empty directory.
// R2.4: -d removes empty directories only.
func removeDirEntry(opts options, path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("cannot remove '%s': %s",
			path, sysErrMsg(err))
	}
	if opts.verbose {
		printRemovedDir(path)
	}
	return nil
}

// printRemoved prints verbose removal output for files.
// R3.3: format matches GNU rm "removed 'PATH'".
func printRemoved(path string) {
	fmt.Fprintf(os.Stdout, "removed '%s'\n", path)
}

// printRemovedDir prints verbose removal output for directories.
// R3.3: format matches GNU rm "removed directory 'PATH'".
func printRemovedDir(path string) {
	fmt.Fprintf(os.Stdout, "removed directory '%s'\n", path)
}

// sysErrMsg extracts the system error message from an os error.
func sysErrMsg(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return capitalizeFirst(pathErr.Err.Error())
	}
	return err.Error()
}

// capitalizeFirst returns s with the first byte uppercased.
func capitalizeFirst(s string) string {
	if len(s) == 0 {
		return s
	}
	if s[0] >= 'a' && s[0] <= 'z' {
		return string(s[0]-32) + s[1:]
	}
	return s
}

// parseArgs separates flags from positional arguments.
func parseArgs(rawArgs []string) (options, []string) {
	var opts options
	var positional []string
	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "--" {
			positional = append(positional, rawArgs[i+1:]...)
			break
		}
		if arg == "--help" {
			printUsage()
			os.Exit(0)
		}
		if arg == "--version" {
			printVersion()
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "--") {
			parseLongFlag(&opts, arg)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			parseShortFlags(&opts, arg)
			continue
		}
		positional = append(positional, arg)
	}
	return opts, positional
}

// parseLongFlag handles long-form flags for rm.
func parseLongFlag(opts *options, arg string) {
	switch arg {
	case "--recursive":
		opts.recursive = true
	case "--force":
		opts.force = true
	case "--dir":
		opts.dir = true
	case "--verbose":
		opts.verbose = true
	}
}

// parseShortFlags handles combined short flags for rm.
// R2.2: -f overrides -i (last-wins for interactive flags).
func parseShortFlags(opts *options, arg string) {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'r', 'R':
			opts.recursive = true
		case 'f':
			opts.force = true
		case 'd':
			opts.dir = true
		case 'v':
			opts.verbose = true
		}
	}
}

// printMissingOperand prints the missing file operand error.
func printMissingOperand() {
	fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
	printTryHelp()
}

// printTryHelp prints the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr,
		"Try '%s --help' for more information.\n", programName)
}

// printUsage prints the usage message.
func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: %s [OPTION]... [FILE]...
Remove (unlink) the FILE(s).

Options:
  -f, --force           ignore nonexistent files and arguments, never prompt
  -r, -R, --recursive   remove directories and their contents recursively
  -d, --dir             remove empty directories
  -v, --verbose         explain what is being done
      --help     display this help and exit
      --version  output version information and exit
`, programName)
}

// printVersion prints version information.
func printVersion() {
	fmt.Fprintf(os.Stdout, "%s 1.0.0\n", programName)
}
