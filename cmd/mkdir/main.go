// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd034-mkdir R1.1–R1.4: basic directory creation and error handling.
// Implements prd034-mkdir R2.1–R2.3: parent directory creation (-p).
// Implements prd034-mkdir R3.1–R3.4: mode setting (-m) and verbose output (-v).
// Implements prd034-mkdir R4.1–R4.3: differential testing, coverage, permission parity.
package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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

// resolveMode parses an octal or symbolic mode string, defaulting to 0777.
// R3.1: supports octal values and symbolic mode strings.
// R3.2: when -m is not given, directories use 0777 modified by umask.
func resolveMode(modeStr string) (os.FileMode, error) {
	if modeStr == "" {
		return 0o777, nil
	}
	// Try octal first.
	val, err := strconv.ParseUint(modeStr, 8, 32)
	if err == nil {
		return os.FileMode(val), nil
	}
	// Try symbolic mode.
	return parseSymbolicMode(modeStr, 0o777)
}

// parseSymbolicMode evaluates a symbolic mode expression against a base mode.
// R3.1: supports symbolic mode strings like "a=rwx", "u=rwx,go=rx", "u+x".
func parseSymbolicMode(expr string, base os.FileMode) (os.FileMode, error) {
	mode := base
	for _, clause := range strings.Split(expr, ",") {
		var err error
		mode, err = applyModeClause(mode, clause)
		if err != nil {
			return 0, err
		}
	}
	return mode, nil
}

// applyModeClause applies a single symbolic mode clause (e.g., "u=rwx")
// to the current mode and returns the updated mode.
func applyModeClause(mode os.FileMode, clause string) (os.FileMode, error) {
	if len(clause) == 0 {
		return 0, fmt.Errorf("invalid mode")
	}
	i, whoMask := parseWho(clause)
	if i >= len(clause) {
		return 0, fmt.Errorf("invalid mode")
	}
	for i < len(clause) {
		op := clause[i]
		if op != '+' && op != '-' && op != '=' {
			return 0, fmt.Errorf("invalid mode")
		}
		i++
		var permBits uint
		permBits, i = parsePermBits(clause, i)
		mode = applyPermOp(mode, whoMask, op, permBits)
	}
	return mode, nil
}

// parseWho parses the who part of a symbolic mode clause (u, g, o, a).
// Returns the index after the who characters and the who mask.
func parseWho(clause string) (int, os.FileMode) {
	var who os.FileMode
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
			return i, who
		}
		i++
	}
	if who == 0 {
		who = 0o777
	}
	return i, who
}

// parsePermBits parses permission characters (r, w, x, X) and returns
// abstract permission bits (r=4, w=2, x=1) and the new index position.
func parsePermBits(clause string, start int) (uint, int) {
	var bits uint
	i := start
	for i < len(clause) {
		switch clause[i] {
		case 'r':
			bits |= 4
		case 'w':
			bits |= 2
		case 'x', 'X':
			// X is conditional execute, but for mkdir (directories) it is
			// equivalent to x since directories always get execute.
			bits |= 1
		default:
			return bits, i
		}
		i++
	}
	return bits, i
}

// applyPermOp applies an operator (+, -, =) with the given permission bits
// to the mode, scoped by the who mask.
func applyPermOp(mode, whoMask os.FileMode, op byte, permBits uint) os.FileMode {
	expanded := expandPerms(whoMask, permBits)
	switch op {
	case '+':
		mode |= expanded
	case '-':
		mode &^= expanded
	case '=':
		mode = (mode &^ whoMask) | expanded
	}
	return mode
}

// expandPerms expands abstract permission bits (r=4, w=2, x=1) into the
// positions specified by the who mask.
func expandPerms(whoMask os.FileMode, permBits uint) os.FileMode {
	var mode os.FileMode
	if whoMask&0o700 != 0 {
		mode |= os.FileMode(permBits << 6)
	}
	if whoMask&0o070 != 0 {
		mode |= os.FileMode(permBits << 3)
	}
	if whoMask&0o007 != 0 {
		mode |= os.FileMode(permBits)
	}
	return mode
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
// R2.3: no error if intermediate directories already exist.
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
