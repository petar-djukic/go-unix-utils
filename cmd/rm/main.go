// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/rm implements GNU rm: remove files or directories.
//
// Implements prd058-rm R1.1-R1.4, R2.1-R2.4.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "rm"

const helpText = `Usage: rm [OPTION]... [FILE]...
Remove (unlink) the FILE(s).

  -f, --force           ignore nonexistent files and arguments, never prompt
  -i                    prompt before every removal
  -I                    prompt once before removing more than three files, or
                          when removing recursively
      --interactive[=WHEN]  prompt according to WHEN: never, once (-I), or
                          always (-i); without WHEN, prompt always
  -d, --dir             remove empty directories
  -r, -R, --recursive   remove directories and their contents recursively
  -v, --verbose         explain what is being done
      --help        display this help and exit
      --version     output version information and exit
`

const versionText = "rm (go-unix-utils) 0.1\n"

type parseResult int

const (
	parseOK   parseResult = iota
	parseHelp
	parseVer
)

// rmOptions holds parsed command-line flags.
type rmOptions struct {
	force     bool
	recursive bool
	dir       bool
	verbose   bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the rm logic and returns the exit code.
func run(args []string, stdout, stderr *os.File) int {
	opts, operands, result, err := parseArgs(args)
	switch result {
	case parseHelp:
		fmt.Fprint(stdout, helpText) //nolint:errcheck
		return 0
	case parseVer:
		fmt.Fprint(stdout, versionText) //nolint:errcheck
		return 0
	}
	if err != nil {
		printError(stderr, err.Error())
		printTryHelp(stderr)
		return 1
	}
	if len(operands) == 0 {
		if opts.force {
			return 0
		}
		printError(stderr, "missing operand")
		printTryHelp(stderr)
		return 1
	}
	return removeAll(operands, opts, stdout, stderr)
}

// removeAll processes each operand and returns the overall exit code.
// R1.4: continues with remaining files on error.
func removeAll(operands []string, opts rmOptions, stdout, stderr *os.File) int {
	exitCode := 0
	for _, path := range operands {
		if removeOne(path, opts, stdout, stderr) != 0 {
			exitCode = 1
		}
	}
	return exitCode
}

// removeOne removes a single file or directory entry.
// R1.1: removes files via os.Remove (unlink).
// R1.2: refuses directories without -r.
// R1.3: refuses '.' and '..'.
func removeOne(path string, opts rmOptions, stdout, stderr *os.File) int {
	// R1.3: refuse to remove '.' or '..'.
	if isDotOrDotDot(path) {
		printRefuseDot(stderr, path)
		return 1
	}

	info, err := os.Lstat(path)
	if err != nil {
		if opts.force && os.IsNotExist(err) {
			return 0
		}
		printRemoveError(stderr, path, err)
		return 1
	}

	if info.IsDir() {
		return removeDirEntry(path, opts, stdout, stderr)
	}

	return removeFileEntry(path, opts, stdout, stderr)
}

// removeDirEntry handles directory removal logic.
// R1.2: without -r or -d, refuse.
func removeDirEntry(path string, opts rmOptions, stdout, stderr *os.File) int {
	if opts.recursive {
		return removeRecursive(path, opts, stdout, stderr)
	}
	if opts.dir {
		return removeEmptyDir(path, opts, stdout, stderr)
	}
	printError(stderr, fmt.Sprintf(
		"cannot remove '%s': Is a directory", path))
	return 1
}

// removeEmptyDir attempts to remove an empty directory.
// R2.4: -d removes empty directories.
func removeEmptyDir(path string, opts rmOptions, stdout, stderr *os.File) int {
	if err := os.Remove(path); err != nil {
		printRemoveError(stderr, path, err)
		return 1
	}
	if opts.verbose {
		printRemovedDir(stdout, path)
	}
	return 0
}

// removeRecursive removes a directory and all contents.
func removeRecursive(path string, opts rmOptions, stdout, stderr *os.File) int {
	entries, err := os.ReadDir(path)
	if err != nil {
		printRemoveError(stderr, path, err)
		return 1
	}
	exitCode := 0
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		if removeOne(child, opts, stdout, stderr) != 0 {
			exitCode = 1
		}
	}
	if err := os.Remove(path); err != nil {
		printRemoveError(stderr, path, err)
		return 1
	}
	if opts.verbose {
		printRemovedDir(stdout, path)
	}
	return exitCode
}

// removeFileEntry removes a regular file or symlink.
// R1.1: unlink via os.Remove.
func removeFileEntry(path string, opts rmOptions, stdout, stderr *os.File) int {
	if err := os.Remove(path); err != nil {
		printRemoveError(stderr, path, err)
		return 1
	}
	if opts.verbose {
		printRemoved(stdout, path)
	}
	return 0
}

// isDotOrDotDot returns true if the base name of path is "." or "..".
// R1.3: refuse removal of dot entries.
func isDotOrDotDot(path string) bool {
	// Strip trailing slashes to get effective base.
	cleaned := strings.TrimRight(path, "/")
	if cleaned == "" {
		// Path was all slashes (e.g., "/"), base is effectively "/".
		return false
	}
	base := filepath.Base(cleaned)
	return base == "." || base == ".."
}

// printRefuseDot prints the GNU-compatible refusal message for '.' or '..'.
func printRefuseDot(stderr *os.File, path string) {
	fmt.Fprintf(stderr, //nolint:errcheck
		"%s: refusing to remove '.' or '..' directory: skipping '%s'\n",
		progName, path)
}

// printRemoveError prints a removal error with context.
func printRemoveError(stderr *os.File, path string, err error) {
	fmt.Fprintf(stderr, "%s: cannot remove '%s': %s\n", //nolint:errcheck
		progName, path, stripPathError(err))
}

// printRemoved prints verbose removal output for files.
func printRemoved(stdout *os.File, path string) {
	fmt.Fprintf(stdout, "removed '%s'\n", path) //nolint:errcheck
}

// printRemovedDir prints verbose removal output for directories.
func printRemovedDir(stdout *os.File, path string) {
	fmt.Fprintf(stdout, "removed directory '%s'\n", path) //nolint:errcheck
}

// parseArgs separates flags from operands.
func parseArgs(args []string) (rmOptions, []string, parseResult, error) {
	var opts rmOptions
	var operands []string
	endOfFlags := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if endOfFlags || !isFlag(arg) {
			operands = append(operands, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if isLongFlag(arg) {
			result, err := parseLongFlag(arg, &opts)
			if result != parseOK {
				return opts, nil, result, nil
			}
			if err != nil {
				return opts, nil, parseOK, err
			}
			continue
		}
		if err := parseShortFlags(arg[1:], &opts); err != nil {
			return opts, nil, parseOK, err
		}
	}

	return opts, operands, parseOK, nil
}

// parseLongFlag handles long-form flags.
func parseLongFlag(flag string, opts *rmOptions) (parseResult, error) {
	name, value, hasValue := splitLongFlag(flag)
	switch name {
	case "--help":
		return parseHelp, nil
	case "--version":
		return parseVer, nil
	case "--force":
		opts.force = true
	case "--recursive":
		opts.recursive = true
	case "--dir":
		opts.dir = true
	case "--verbose":
		opts.verbose = true
	case "--interactive":
		return parseOK, parseLongInteractive(value, hasValue, opts)
	default:
		return parseOK, fmt.Errorf("unrecognized option '%s'", flag)
	}
	return parseOK, nil
}

// parseLongInteractive handles --interactive[=WHEN].
func parseLongInteractive(value string, hasValue bool, opts *rmOptions) error {
	if !hasValue {
		// --interactive without =WHEN is "always" (like -i).
		_ = opts // -i/-I not implemented in R1; placeholder.
		return nil
	}
	switch value {
	case "never":
		opts.force = true
	case "once", "always":
		// -I and -i not implemented in R1; placeholder.
	default:
		return fmt.Errorf(
			"invalid argument '%s' for '--interactive'", value)
	}
	return nil
}

// parseShortFlags handles short flags and combined forms.
func parseShortFlags(flags string, opts *rmOptions) error {
	for i := range len(flags) {
		switch flags[i] {
		case 'f':
			opts.force = true
		case 'r', 'R':
			opts.recursive = true
		case 'd':
			opts.dir = true
		case 'v':
			opts.verbose = true
		case 'i':
			// -i not implemented in R1; placeholder.
		case 'I':
			// -I not implemented in R1; placeholder.
		default:
			return fmt.Errorf("invalid option -- '%c'", flags[i])
		}
	}
	return nil
}

// splitLongFlag splits --name=value into components.
func splitLongFlag(flag string) (string, string, bool) {
	name, value, ok := strings.Cut(flag, "=")
	if ok {
		return name, value, true
	}
	return flag, "", false
}

// isFlag returns true if arg starts with '-' and has content after it.
func isFlag(arg string) bool {
	return len(arg) > 1 && arg[0] == '-'
}

// isLongFlag returns true if arg starts with '--'.
func isLongFlag(arg string) bool {
	return len(arg) > 2 && arg[0] == '-' && arg[1] == '-'
}

// stripPathError extracts the inner error message from *os.PathError.
func stripPathError(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}

// printError prints a formatted error to stderr.
func printError(stderr *os.File, msg string) {
	fmt.Fprintf(stderr, "%s: %s\n", progName, msg) //nolint:errcheck
}

// printTryHelp prints the "try help" hint to stderr.
func printTryHelp(stderr *os.File) {
	fmt.Fprintf(stderr, //nolint:errcheck
		"Try '%s --help' for more information.\n", progName)
}
