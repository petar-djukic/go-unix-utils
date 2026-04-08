// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/printenv: print environment variables.
// Implements srd040-printenv R1.1, R1.2, R1.3, R2.1.
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "printenv"

const helpText = `Usage: printenv [OPTION]... [VARIABLE]...
Print the values of the specified environment VARIABLE(s).
If no VARIABLE is specified, print name and value pairs for them all.

  -0, --null     end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit
`

const versionText = progName + " (go-unix-utils)"

func main() {
	sys.InstallSIGPIPEHandler()

	code := run(os.Args[1:])
	os.Exit(code)
}

// printenvOptions holds parsed flag state for printenv.
type printenvOptions struct {
	nullTerm bool
	names    []string
}

// run parses arguments and prints the requested environment variables.
// Returns the exit code.
func run(args []string) int {
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		return 2
	}

	if len(opts.names) == 0 {
		return printAll(opts.nullTerm)
	}
	return printNamed(opts.names, opts.nullTerm)
}

// printAll prints every environment variable in NAME=VALUE format. R1.1, R2.4.
func printAll(nullTerm bool) int {
	sep := terminator(nullTerm)
	for _, e := range os.Environ() {
		fmt.Print(e + sep)
	}
	return 0
}

// printNamed prints values of named variables. R1.2, R1.3, R2.2, R2.3.
func printNamed(names []string, nullTerm bool) int {
	sep := terminator(nullTerm)
	allFound := true
	for _, name := range names {
		val, ok := os.LookupEnv(name)
		if !ok {
			allFound = false
			continue
		}
		fmt.Print(val + sep)
	}
	if !allFound {
		return 1
	}
	return 0
}

// terminator returns the line terminator based on the nullTerm flag. R2.1.
func terminator(nullTerm bool) string {
	if nullTerm {
		return "\x00"
	}
	return "\n"
}

// parseArgs separates flags from VARIABLE name arguments.
func parseArgs(args []string) (printenvOptions, error) {
	var opts printenvOptions
	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--help" {
			fmt.Print(helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println(versionText)
			os.Exit(0)
		}
		if arg == "-0" || arg == "--null" {
			opts.nullTerm = true
			i++
			continue
		}
		if arg == "--" {
			i++
			break
		}
		if len(arg) > 1 && arg[0] == '-' {
			return printenvOptions{}, fmt.Errorf("unrecognized option '%s'", arg)
		}
		break
	}
	opts.names = args[i:]
	return opts, nil
}
