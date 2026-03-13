// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd035-rmdir R1.1–R1.4, R2.1–R2.3, R3.1–R3.4
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "rmdir"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string
	// R2.1: -p / --parents flag for recursive parent removal.
	parents := false
	// R3.1: --ignore-fail-on-non-empty suppresses non-empty directory errors.
	ignoreNonEmpty := false
	// R3.3: -v / --verbose prints a message for each removed directory.
	verbose := false

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			printHelp()
			return
		case arg == "--version":
			printVersion()
			return
		case arg == "--parents":
			parents = true
		case arg == "--ignore-fail-on-non-empty":
			ignoreNonEmpty = true
		case arg == "--verbose":
			verbose = true
		case arg == "--":
			// End of flags; remaining args are operands.
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// Unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Parse bundled short options.
			flags := arg[1:]
			for j := 0; j < len(flags); j++ {
				switch flags[j] {
				case 'p':
					parents = true
				case 'v':
					verbose = true
				default:
					fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, flags[j])
					fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
					os.Exit(1)
				}
			}
		default:
			operands = append(operands, arg)
		}
	}

	// R1.1, R1.2: at least one operand is required.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	exitCode := 0
	for _, dir := range operands {
		// R3.3: verbose output is printed before the removal attempt, matching GNU behavior.
		if verbose {
			fmt.Fprintf(os.Stdout, "%s: removing directory, '%s'\n", programName, dir)
		}
		// R1.1: remove a single empty directory.
		// R1.2, R2.3: process each operand independently.
		// R1.4: reject non-directory targets with "Not a directory".
		if err := syscall.Rmdir(dir); err != nil {
			// R3.1: suppress error when directory is not empty and --ignore-fail-on-non-empty is set.
			// R3.2: do not suppress errors for other failure reasons.
			if !(ignoreNonEmpty && isNonEmptyError(err)) {
				fmt.Fprintf(os.Stderr, "%s: failed to remove '%s': %s\n", programName, dir, err.Error())
				exitCode = 1
			}
			continue
		}
		// R2.1: with -p, ascend and remove each successive empty parent.
		// R2.2: stop ascending when a parent is not empty and report an error.
		if parents {
			parent := dir
			for {
				parent = filepath.Dir(parent)
				if parent == "." || parent == "/" {
					break
				}
				// R3.3: verbose output for each ancestor, before the removal attempt.
				if verbose {
					fmt.Fprintf(os.Stdout, "%s: removing directory, '%s'\n", programName, parent)
				}
				if err := syscall.Rmdir(parent); err != nil {
					// R3.4: with -p and --ignore-fail-on-non-empty, suppress non-empty ancestor errors.
					if !(ignoreNonEmpty && isNonEmptyError(err)) {
						fmt.Fprintf(os.Stderr, "%s: failed to remove '%s': %s\n", programName, parent, err.Error())
						exitCode = 1
					}
					break
				}
			}
		}
	}

	os.Exit(exitCode)
}

// isNonEmptyError reports whether err indicates that a directory removal
// failed because the directory was not empty.
//
// R3.1: used to decide whether --ignore-fail-on-non-empty should suppress the error.
func isNonEmptyError(err error) bool {
	return err == syscall.ENOTEMPTY || err == syscall.EEXIST
}

// printHelp writes usage information to stdout and exits 0.
//
// R1.4: --help prints usage to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: rmdir [OPTION]... DIRECTORY...
Remove the DIRECTORY(ies), if they are empty.

      --ignore-fail-on-non-empty
                   ignore each failure that is solely because a directory
                   is non-empty
  -p, --parents    remove DIRECTORY and its ancestors
  -v, --verbose    output a diagnostic for every directory processed
      --help       display this help and exit
      --version    output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
//
// R1.3: --version prints version info to stdout and exits 0.
func printVersion() {
	fmt.Println("rmdir (go-unix-utils) 0.1")
}
