// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd016-dirname R1.1–R1.5, R2.1–R2.2, R3.1–R3.3, R4.1–R4.2:
// strip last component from file paths with error handling, edge cases,
// and --version/--help support.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set via ldflags at build time.
var version = "unknown"

// options holds parsed command-line flags.
type options struct {
	zero    bool
	version bool
}

func main() {
	sys.InstallSIGPIPEHandler()
	opts, names := parseArgs(os.Args[1:])
	// R4.1: --version prints version information and exits 0.
	if opts.version {
		fmt.Printf("dirname (go-unix-utils) %s\n", version)
		return
	}
	// R3.2: exit 1 with error to stderr when no arguments given.
	if len(names) == 0 {
		printError("missing operand")
		os.Exit(1)
	}
	terminator := "\n"
	if opts.zero {
		terminator = "\x00"
	}
	// R1.5, R2.2: process multiple arguments in order.
	// R3.1: exit 0 on success. R3.3: exit 1 on stdout write error.
	for _, name := range names {
		if _, err := fmt.Print(dirname(name) + terminator); err != nil {
			os.Exit(1)
		}
	}
}

// parseArgs splits raw arguments into options and positional names.
// Supports -z, --zero, --version, --help, and -- to end option parsing.
func parseArgs(args []string) (options, []string) {
	var opts options
	var names []string
	for i := range len(args) {
		arg := args[i]
		if arg == "--" {
			names = append(names, args[i+1:]...)
			break
		}
		if arg == "--help" {
			printUsage()
			os.Exit(0)
		}
		if arg == "--version" {
			opts.version = true
			return opts, nil
		}
		if arg == "--zero" {
			opts.zero = true
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 && !strings.HasPrefix(arg, "--") {
			for j := 1; j < len(arg); j++ {
				if arg[j] == 'z' {
					opts.zero = true
				}
			}
			continue
		}
		names = append(names, arg)
	}
	return opts, names
}

// printUsage writes usage information to stdout. Implements R4.2.
func printUsage() {
	fmt.Print("Usage: dirname [OPTION] NAME...\n" +
		"Output each NAME with its last non-slash component " +
		"and trailing slashes removed;\n" +
		"if NAME contains no /'s, output '.' " +
		"(meaning the current directory).\n\n" +
		"  -z, --zero     end each output line with NUL, not newline\n" +
		"      --help     display this help and exit\n" +
		"      --version  output version information and exit\n")
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr,
		"dirname: %s\nTry 'dirname --help' for more information.\n", msg)
}

// dirname extracts the directory component from name, matching GNU
// coreutils behavior. Implements R1.1–R1.4.
func dirname(name string) string {
	if name == "" {
		return "."
	}
	if allSlashes(name) {
		return "/"
	}
	// R1.1: strip trailing slashes before extracting directory.
	name = strings.TrimRight(name, "/")
	i := strings.LastIndex(name, "/")
	if i < 0 {
		// R1.2: no slash means current directory.
		return "."
	}
	// R1.4: strip trailing slashes from the result.
	dir := strings.TrimRight(name[:i], "/")
	if dir == "" {
		return "/"
	}
	return dir
}

// allSlashes reports whether s is non-empty and consists entirely of '/'.
func allSlashes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] != '/' {
			return false
		}
	}
	return true
}
