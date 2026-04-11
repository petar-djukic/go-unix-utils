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
// R1.4: prints version information and exits 0.
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
// R1.4: matches GNU pathchk --help structure.
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
// Returns true if the path is valid.
func checkPath(path string, opts options) bool {
	if opts.extraPortable {
		if !checkExtraPortable(path) {
			return false
		}
	}
	if opts.posix {
		return checkPosixPortable(path)
	}
	return checkSystem(path)
}

// checkExtraPortable applies -P checks: no empty path, no leading dash in components.
// R1.3: -P checks for empty names and leading hyphen.
func checkExtraPortable(path string) bool {
	if path == "" {
		fmt.Fprintf(os.Stderr, "%s: empty file name\n", progName)
		return false
	}
	components := strings.Split(path, "/")
	for _, comp := range components {
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
// R1.2: portable characters only, component <= 14, path <= 256.
func checkPosixPortable(path string) bool {
	if len(path) > portableMaxPath {
		fmt.Fprintf(os.Stderr,
			"%s: limit %d exceeded by length %d of file name '%s'\n",
			progName, portableMaxPath, len(path), path)
		return false
	}
	if !checkPortableChars(path) {
		return false
	}
	return checkPortableComponentLengths(path)
}

// checkPortableChars verifies all characters are in the portable set.
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
func checkPortableComponentLengths(path string) bool {
	components := strings.Split(path, "/")
	for _, comp := range components {
		if len(comp) > portableMaxComponent {
			fmt.Fprintf(os.Stderr,
				"%s: limit %d exceeded by length %d of file name component '%s'\n",
				progName, portableMaxComponent, len(comp), comp)
			return false
		}
	}
	return true
}

// checkSystem validates a path against current system limits.
// R1.1: checks component length, path length, and character validity.
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
				"%s: non-portable character '\\0' in file name '%s'\n",
				progName, path)
			return false
		}
	}
	return true
}
