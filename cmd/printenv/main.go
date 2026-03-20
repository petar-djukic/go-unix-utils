// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd040-printenv R1.1, R1.2, R1.3, R2.1:
// printenv basic variable display, NUL-delimited output, and exit codes.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is the program name used in error messages.
const progName = "printenv"

func main() {
	sys.InstallSIGPIPEHandler()
	args := os.Args[1:]
	useNull, vars := parseArgs(args)
	if len(vars) == 0 {
		printAllVars(useNull)
		return
	}
	os.Exit(printNamedVars(vars, useNull))
}

// parseArgs separates option flags from variable name arguments.
// Returns whether NUL termination is enabled and the remaining args.
func parseArgs(args []string) (bool, []string) {
	useNull := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return useNull, args[i+1:]
		case arg == "--null":
			useNull = true
		case arg == "--help":
			printAndExit(helpText)
		case arg == "--version":
			printAndExit(versionText)
		case strings.HasPrefix(arg, "--"):
			exitInvalidOption(arg)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			useNull = parseShortFlags(arg[1:], useNull)
		default:
			return useNull, args[i:]
		}
	}
	return useNull, nil
}

// parseShortFlags handles combined short flags like -0. Returns
// updated useNull value. Exits on invalid flag characters.
func parseShortFlags(chars string, useNull bool) bool {
	for _, c := range chars {
		switch c {
		case '0':
			useNull = true
		default:
			exitInvalidShortOption(byte(c))
		}
	}
	return useNull
}

// printAllVars prints every environment variable in NAME=VALUE format.
// R1.1: no arguments prints all variables and exits 0.
func printAllVars(useNull bool) {
	terminator := "\n"
	if useNull {
		terminator = "\x00"
	}
	for _, e := range os.Environ() {
		fmt.Print(e + terminator)
	}
}

// printNamedVars prints the value of each named variable. Returns 0 if
// all variables are found, 1 if any are missing.
// R1.2: print only values. R1.3: no output or error for missing vars.
func printNamedVars(vars []string, useNull bool) int {
	terminator := "\n"
	if useNull {
		terminator = "\x00"
	}
	exitCode := 0
	for _, name := range vars {
		val, ok := os.LookupEnv(name)
		if !ok {
			exitCode = 1
			continue
		}
		fmt.Print(val + terminator)
	}
	return exitCode
}

// exitInvalidOption prints an error for an unrecognized long option
// and exits 2 to match GNU printenv behavior.
func exitInvalidOption(opt string) {
	fmt.Fprintf(os.Stderr,
		"%s: unrecognized option '%s'\nTry '%s --help' for more information.\n",
		progName, opt, progName)
	os.Exit(2)
}

// exitInvalidShortOption prints an error for an invalid short option
// character and exits 2 to match GNU printenv behavior.
func exitInvalidShortOption(c byte) {
	fmt.Fprintf(os.Stderr,
		"%s: invalid option -- '%c'\nTry '%s --help' for more information.\n",
		progName, c, progName)
	os.Exit(2)
}

// printAndExit writes text to stdout and exits 0.
func printAndExit(text string) {
	fmt.Fprint(os.Stdout, text)
	os.Exit(0)
}

// helpText is the usage message printed for --help.
const helpText = `Usage: printenv [OPTION]... [VARIABLE]...
Print the values of the specified environment VARIABLE(s).
If no VARIABLE is specified, print name and value pairs for them all.

  -0, --null     end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit
`

// versionText is the version message printed for --version.
const versionText = `printenv (go-unix-utils) 1.0
`
