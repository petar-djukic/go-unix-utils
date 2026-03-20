// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd034-mkdir R1.1–R1.4: basic directory creation and error handling.
// Implements prd034-mkdir R2.1–R2.3: parent directory creation (-p).
// Implements prd034-mkdir R3.1–R3.4: mode setting (-m) and verbose output (-v).
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "mkdir"

// options holds parsed GNU mkdir flags.
type options struct {
	parents bool   // -p, --parents (R2.1)
	verbose bool   // -v, --verbose (R3.4)
	mode    string // -m, --mode (R3.1)
}

func main() {
	sys.InstallSIGPIPEHandler()
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

// run parses flags and creates directories, returning the exit code.
// R1.1: creates directories from command-line arguments.
// R1.2: processes multiple directory arguments independently.
func run(args []string, stdout, stderr io.Writer) int {
	opts, dirs, code := parseArgs(args, stdout, stderr)
	if code >= 0 {
		return code
	}
	if len(dirs) == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", progName)
		printTryHelp(stderr)
		return 1
	}
	return createDirs(dirs, opts, stdout, stderr)
}

// parseArgs separates flags from directory arguments.
// Returns parsed options, directory list, and exit code (-1 = continue).
func parseArgs(args []string, stdout, stderr io.Writer) (options, []string, int) {
	var opts options
	var dirs []string
	flagsDone := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if flagsDone || len(arg) == 0 || arg[0] != '-' {
			dirs = append(dirs, arg)
			continue
		}
		if arg == "--" {
			flagsDone = true
			continue
		}
		if len(arg) > 2 && arg[1] == '-' {
			code := applyLongFlag(&opts, arg, args, &i, stdout, stderr)
			if code >= 0 {
				return opts, nil, code
			}
			continue
		}
		code := applyShortFlags(&opts, arg, args, &i, stderr)
		if code >= 0 {
			return opts, nil, code
		}
	}
	return opts, dirs, -1
}

// applyShortFlags processes combined short flags (e.g., -pv).
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyShortFlags(opts *options, arg string, args []string, idx *int, stderr io.Writer) int {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case 'p':
			opts.parents = true
		case 'v':
			opts.verbose = true
		case 'm':
			return consumeModeValue(opts, arg[j+1:], args, idx, stderr)
		default:
			fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", progName, arg[j])
			printTryHelp(stderr)
			return 1
		}
	}
	return -1
}

// consumeModeValue reads the -m value from the rest of the arg or the next arg.
func consumeModeValue(opts *options, rest string, args []string, idx *int, stderr io.Writer) int {
	if rest != "" {
		opts.mode = rest
		return -1
	}
	if *idx+1 < len(args) {
		*idx++
		opts.mode = args[*idx]
		return -1
	}
	fmt.Fprintf(stderr, "%s: option requires an argument -- 'm'\n", progName)
	printTryHelp(stderr)
	return 1
}

// applyLongFlag handles --long-name flags.
// Returns exit code >= 0 for terminal flags, -1 to continue.
func applyLongFlag(opts *options, arg string, args []string, idx *int, stdout, stderr io.Writer) int {
	switch {
	case arg == "--parents":
		opts.parents = true
	case arg == "--verbose":
		opts.verbose = true
	case arg == "--mode" || hasPrefix(arg, "--mode="):
		return consumeLongMode(opts, arg, args, idx, stderr)
	case arg == "--help":
		printHelp(stdout)
		return 0
	case arg == "--version":
		printVersion(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", progName, arg)
		printTryHelp(stderr)
		return 1
	}
	return -1
}

// consumeLongMode reads the --mode value from =suffix or next arg.
func consumeLongMode(opts *options, arg string, args []string, idx *int, stderr io.Writer) int {
	eqForm := "--mode="
	if hasPrefix(arg, eqForm) {
		opts.mode = arg[len(eqForm):]
		return -1
	}
	if *idx+1 < len(args) {
		*idx++
		opts.mode = args[*idx]
		return -1
	}
	fmt.Fprintf(stderr, "%s: option '--mode' requires an argument\n", progName)
	printTryHelp(stderr)
	return 1
}

// hasPrefix returns true if s starts with prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// createDirs creates each directory, returning 0 on success or 1 on any failure.
func createDirs(dirs []string, opts options, stdout, stderr io.Writer) int {
	mode, err := resolveMode(opts.mode)
	if err != nil {
		fmt.Fprintf(stderr, "%s: invalid mode '%s'\n", progName, opts.mode)
		return 1
	}
	exitCode := 0
	for _, dir := range dirs {
		if err := createOne(dir, opts, mode, stdout); err != nil {
			fmt.Fprintf(stderr, "%s: cannot create directory '%s': %s\n", progName, dir, err)
			exitCode = 1
		}
	}
	return exitCode
}

// resolveMode parses an octal mode string, defaulting to 0777.
// R3.2: when -m is not given, directories use 0777 modified by umask.
func resolveMode(modeStr string) (os.FileMode, error) {
	if modeStr == "" {
		return 0o777, nil
	}
	val, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(val), nil
}

// createOne dispatches to parent or single creation based on -p flag.
func createOne(dir string, opts options, mode os.FileMode, stdout io.Writer) error {
	if opts.parents {
		return createWithParents(dir, opts, mode, stdout)
	}
	return createSingle(dir, opts, mode, stdout)
}

// createSingle creates a single directory without parent creation.
// R1.3: errors if directory already exists.
// R1.4: errors if parent does not exist.
func createSingle(dir string, opts options, mode os.FileMode, stdout io.Writer) error {
	if err := os.Mkdir(dir, mode); err != nil {
		return unwrapPathError(err)
	}
	if opts.mode != "" {
		// R3.1: chmod to exact mode since os.Mkdir applies umask.
		if err := os.Chmod(dir, mode); err != nil {
			return unwrapPathError(err)
		}
	}
	if opts.verbose {
		fmt.Fprintf(stdout, "%s: created directory '%s'\n", progName, dir)
	}
	return nil
}

// createWithParents creates a directory and any missing ancestors.
// R2.1: creates intermediate directories as needed.
// R2.2: no error if target already exists.
// R3.3: applies -m mode only to the final target.
func createWithParents(dir string, opts options, mode os.FileMode, stdout io.Writer) error {
	toCreate := dirsToCreate(dir)
	if len(toCreate) == 0 {
		return nil // R2.2: all directories already exist
	}
	for _, d := range toCreate {
		if err := mkdirOne(d, dir, opts, mode, stdout); err != nil {
			return err
		}
	}
	return nil
}

// mkdirOne creates a single directory within a -p chain.
func mkdirOne(d, target string, opts options, mode os.FileMode, stdout io.Writer) error {
	isTarget := d == target
	dirMode := os.FileMode(0o777)
	if isTarget && opts.mode != "" {
		dirMode = mode
	}
	if err := os.Mkdir(d, dirMode); err != nil {
		return unwrapPathError(err)
	}
	if isTarget && opts.mode != "" {
		if err := os.Chmod(d, mode); err != nil {
			return unwrapPathError(err)
		}
	}
	if opts.verbose {
		fmt.Fprintf(stdout, "%s: created directory '%s'\n", progName, d)
	}
	return nil
}

// dirsToCreate returns directories that need creating, from outermost
// ancestor to the target, skipping those that already exist.
func dirsToCreate(dir string) []string {
	var components []string
	current := dir
	for current != "" && current != "/" && current != "." {
		if _, err := os.Stat(current); err == nil {
			break
		}
		components = append(components, current)
		current = parentDir(current)
	}
	// Reverse: create outermost first.
	for i, j := 0, len(components)-1; i < j; i, j = i+1, j-1 {
		components[i], components[j] = components[j], components[i]
	}
	return components
}

// parentDir returns the parent directory of path.
func parentDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return "."
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp(w io.Writer) {
	fmt.Fprintf(w, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to w.
func printHelp(w io.Writer) {
	fmt.Fprintf(w, "Usage: %s [OPTION]... DIRECTORY...\n", progName)
	fmt.Fprintln(w, "Create the DIRECTORY(ies), if they do not already exist.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  -m, --mode=MODE   set file mode (as in chmod), not a=rwx - umask")
	fmt.Fprintln(w, "  -p, --parents     no error if existing, make parent directories as needed")
	fmt.Fprintln(w, "  -v, --verbose     print a message for each created directory")
	fmt.Fprintln(w, "      --help        display this help and exit")
	fmt.Fprintln(w, "      --version     output version information and exit")
}

// printVersion writes version information to w.
func printVersion(w io.Writer) {
	fmt.Fprintf(w, "%s (go-unix-utils)\n", progName)
}

// unwrapPathError extracts the inner error from *os.PathError for
// GNU-compatible error messages (e.g., "No such file or directory").
func unwrapPathError(err error) error {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err
	}
	return err
}
