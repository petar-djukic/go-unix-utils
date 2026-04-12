// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/dirname: strip last component from file paths.
// Implements srd016-dirname R1.1-R1.5, R2.1, R2.2, R4.1, R4.2, R4.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "dirname"

// versionText is printed when --version is passed.
// R4.1: prints version information and exits 0.
const versionText = progName + " (go-unix-utils) dev"

func main() {
	// R4.3: install SIGPIPE handler to exit cleanly when piped to head, etc.
	sys.InstallSIGPIPEHandler()

	// R4.1/R4.2: handle --version and --help before argument parsing.
	if handleInfoFlags(os.Args[1:]) {
		return
	}

	opts, names, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	terminator := "\n"
	if opts.zero {
		terminator = "\x00"
	}

	// R1.5: process multiple NAME arguments in order.
	// R2.2: results printed in argument order.
	for _, arg := range names {
		fmt.Print(dirname(arg) + terminator)
	}
}

// handleInfoFlags checks for --version and --help, prints and exits 0.
// Returns true if a flag was handled (caller should return).
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
// R4.2: matches GNU dirname --help structure.
func printHelp() {
	fmt.Print(`Usage: dirname [OPTION] NAME...
Output each NAME with its last non-slash component and trailing slashes removed;
if NAME contains no /'s, output '.' (meaning the current directory).

  -z, --zero     end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit
`)
}

// options holds parsed command-line flags.
type options struct {
	zero bool
}

// parseArgs parses flags and positional arguments.
// Returns options, the list of NAME arguments, and any error.
func parseArgs(args []string) (options, []string, error) {
	var opts options
	var names []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			names = append(names, args[i+1:]...)
			break
		}
		switch arg {
		case "-z", "--zero":
			opts.zero = true
		default:
			names = append(names, args[i:]...)
			i = len(args) // break outer loop
		}
	}

	if len(names) == 0 {
		return opts, nil, fmt.Errorf("%s: missing operand", progName)
	}
	return opts, names, nil
}

// dirname extracts the directory component from a pathname.
// R1.1: strip trailing slashes, then remove everything after the last '/'.
// R1.2: if no '/' remains after trailing-slash removal, return ".".
// R1.3: if the name is entirely slashes, return "/".
// R1.4: strip trailing slashes from the result; if empty, return "/".
func dirname(name string) string {
	// R1.2: empty string has no slash, return ".".
	if name == "" {
		return "."
	}

	// R1.3: name consisting entirely of slashes returns "/".
	trimmed := strings.TrimRight(name, "/")
	if trimmed == "" {
		return "/"
	}

	// R1.1: find the last slash to split directory from base.
	idx := strings.LastIndex(trimmed, "/")
	if idx < 0 {
		// R1.2: no slash means current directory.
		return "."
	}

	// R1.4: strip trailing slashes from the directory portion.
	dir := strings.TrimRight(trimmed[:idx], "/")
	if dir == "" {
		return "/"
	}
	return dir
}
