// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/pathchk: check whether file names are valid or portable.
// Implements srd103-pathchk R1.1-R1.4, R2.1-R2.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "pathchk"

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils) dev"

// portableMaxComponent is the POSIX _POSIX_NAME_MAX limit for -p mode.
const portableMaxComponent = 14

// portableMaxPath is the POSIX _POSIX_PATH_MAX limit for -p mode.
const portableMaxPath = 256

// portableChars is the set of characters allowed by POSIX portable filename character set.
const portableChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789._-"

func main() {
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	if handleInfoFlags(args) {
		return
	}

	opts, paths, err := parseArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", os.Args[0])
		os.Exit(1)
	}

	if len(paths) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", progName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", os.Args[0])
		os.Exit(1)
	}

	// R2.1: exit 0 when all pathnames pass.
	// R2.2: exit 1 when any pathname fails.
	exitCode := 0
	for _, p := range paths {
		if !checkPath(p, opts) {
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

// options holds parsed command-line flags.
type options struct {
	posix         bool // -p: POSIX portable filename checking
	extraPortable bool // -P: additional portability checks
}

// handleInfoFlags checks for --version and --help, prints and exits 0.
func handleInfoFlags(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--version":
			fmt.Println(versionText)
			return true
		case "--help":
			printHelp()
			return true
		case "--":
			return false
		}
	}
	return false
}

// printHelp writes usage information to stdout.
func printHelp() {
	fmt.Print(`Usage: pathchk [OPTION]... NAME...
Diagnose invalid or unportable file names.

  -p                  check for most POSIX systems
  -P                  check for empty names and leading "-"
      --portability   check for all POSIX systems (equivalent to -p -P)
      --help        display this help and exit
      --version     output version information and exit
`)
}

// parseArgs separates flags from pathname arguments.
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var paths []string
	endOfFlags := false

	for _, arg := range args {
		if endOfFlags {
			paths = append(paths, arg)
			continue
		}
		if arg == "--" {
			endOfFlags = true
			continue
		}
		if arg == "--portability" {
			opts.posix = true
			opts.extraPortable = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			if err := parseFlagArg(arg, &opts); err != nil {
				return opts, nil, err
			}
			continue
		}
		paths = append(paths, arg)
		endOfFlags = true
	}
	return opts, paths, nil
}

// parseFlagArg handles short flags like -p, -P, -pP.
func parseFlagArg(arg string, opts *options) error {
	for i := 1; i < len(arg); i++ {
		switch arg[i] {
		case 'p':
			opts.posix = true
		case 'P':
			opts.extraPortable = true
		default:
			return fmt.Errorf("%s: invalid option -- '%c'",
				progName, arg[i])
		}
	}
	return nil
}

// checkPath validates a single pathname according to the given options.
// R2.1: returns true when the path passes all applicable checks.
// R2.2: returns false when any check fails.
// Matches GNU pathchk order: posix checks first, then extra portability.
// Both checks run independently when both flags are set.
func checkPath(path string, opts options) bool {
	if !opts.posix && !opts.extraPortable {
		return checkSystem(path)
	}
	ok := true
	if opts.posix && !checkPosixPortable(path) {
		ok = false
	}
	if opts.extraPortable && !checkExtraPortable(path) {
		ok = false
	}
	return ok
}

// checkExtraPortable applies -P checks: no empty path, no leading dash in components.
// R1.3: -P checks for empty names and leading hyphen.
func checkExtraPortable(path string) bool {
	if path == "" {
		fmt.Fprintf(os.Stderr, "%s: empty file name\n", progName)
		return false
	}
	for _, comp := range strings.Split(path, "/") {
		if strings.HasPrefix(comp, "-") {
			fmt.Fprintf(os.Stderr,
				"%s: leading '-' in a component of file name '%s'\n",
				progName, path)
			return false
		}
	}
	return true
}

// checkPosixPortable validates against POSIX portable filename rules.
// R1.2: portable characters only, component <= 14, path < _POSIX_PATH_MAX.
// R2.1/R2.2: reports all violations without short-circuiting, matching GNU behavior.
func checkPosixPortable(path string) bool {
	if path == "" {
		fmt.Fprintf(os.Stderr, "%s: empty file name\n", progName)
		return false
	}
	ok := true
	if len(path) >= portableMaxPath {
		fmt.Fprintf(os.Stderr,
			"%s: limit %d exceeded by length %d of file name '%s'\n",
			progName, portableMaxPath-1, len(path), path)
		ok = false
	}
	if !checkPortableChars(path) {
		ok = false
	}
	if !checkPortableComponentLengths(path) {
		ok = false
	}
	return ok
}

// checkPortableChars verifies all characters are in the portable set.
// Reports the first nonportable character found.
func checkPortableChars(path string) bool {
	for _, ch := range path {
		if ch == '/' {
			continue
		}
		if !strings.ContainsRune(portableChars, ch) {
			fmt.Fprintf(os.Stderr,
				"%s: non-portable character '%c' in file name '%s'\n",
				progName, ch, path)
			return false
		}
	}
	return true
}

// checkPortableComponentLengths checks each component against the 14-char limit.
// Reports all components that exceed the limit, matching GNU behavior.
func checkPortableComponentLengths(path string) bool {
	ok := true
	for _, comp := range strings.Split(path, "/") {
		if len(comp) > portableMaxComponent {
			fmt.Fprintf(os.Stderr,
				"%s: limit %d exceeded by length %d of file name component '%s'\n",
				progName, portableMaxComponent, len(comp), comp)
			ok = false
		}
	}
	return ok
}

// checkSystem validates a path against current system limits.
// R1.1: checks for empty path and NUL characters.
func checkSystem(path string) bool {
	if path == "" {
		fmt.Fprintf(os.Stderr, "%s: '': No such file or directory\n", progName)
		return false
	}
	return checkSystemChars(path)
}

// checkSystemChars checks for NUL bytes in the path (invalid on all POSIX systems).
func checkSystemChars(path string) bool {
	for _, ch := range path {
		if ch == 0 {
			fmt.Fprintf(os.Stderr,
				"%s: nonportable character '\\0' in file name '%s'\n",
				progName, path)
			return false
		}
	}
	return true
}
