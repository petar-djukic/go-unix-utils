// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/dircolors implements GNU dircolors — outputs shell commands to set LS_COLORS.
// Implements srd109 R1.1–R1.4, R2.1–R2.5, R3.1–R3.5.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "dircolors"

// shellFormat selects the output format.
type shellFormat int

const (
	shellBourne shellFormat = iota
	shellCsh
)

// options holds parsed command-line state.
type options struct {
	format        shellFormat
	explicitShell bool
	printDB       bool
	filename      string
}

func main() {
	sys.InstallSIGPIPEHandler()

	opts, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
}

// run executes the main logic based on parsed options.
func run(opts options) error {
	// R3.2: -p is incompatible with a filename argument.
	if opts.printDB && opts.filename != "" {
		return fmt.Errorf(
			"the options to output dircolors' internal database and\n" +
				"to select a shell syntax are mutually exclusive")
	}

	// R3.1: --print-database prints the built-in database.
	if opts.printDB {
		fmt.Print(builtinDatabase)
		return nil
	}

	entries, err := loadDatabase(opts.filename)
	if err != nil {
		return err
	}

	value := buildLSColors(entries)
	printShellOutput(opts.format, value)
	return nil
}

// parseArgs processes flags and returns the parsed options.
// R1.4: -b and -c are mutually exclusive; last one wins.
func parseArgs(args []string) (options, error) {
	var opts options

	for i := range args {
		arg := args[i]
		switch arg {
		case "-b", "--sh", "--bourne-shell":
			opts.format = shellBourne
			opts.explicitShell = true
		case "-c", "--csh", "--c-shell":
			opts.format = shellCsh
			opts.explicitShell = true
		case "-p", "--print-database":
			opts.printDB = true
		case "--help":
			printUsage()
			os.Exit(0)
		case "--version":
			fmt.Println(programName)
			os.Exit(0)
		default:
			// R2.5: "-" is a valid filename meaning stdin, not an option.
			if arg != "-" && strings.HasPrefix(arg, "-") {
				return options{}, fmt.Errorf("unrecognized option '%s'", arg)
			}
			if opts.filename != "" {
				return options{}, fmt.Errorf("extra operand '%s'", arg)
			}
			opts.filename = arg
		}
	}

	// R1.3: auto-detect from SHELL env when no explicit flag given.
	if !opts.explicitShell {
		opts.format = detectShell()
	}
	return opts, nil
}

// detectShell checks SHELL env var; if it ends with "csh", use C shell format.
func detectShell() shellFormat {
	shell := os.Getenv("SHELL")
	base := shell
	if idx := strings.LastIndex(shell, "/"); idx >= 0 {
		base = shell[idx+1:]
	}
	if strings.HasSuffix(base, "csh") {
		return shellCsh
	}
	return shellBourne
}

// printShellOutput writes the LS_COLORS assignment in the selected format.
func printShellOutput(format shellFormat, value string) {
	switch format {
	case shellCsh:
		// R1.2: setenv LS_COLORS '<value>'
		fmt.Printf("setenv LS_COLORS '%s'\n", value)
	default:
		// R1.1: LS_COLORS='<value>';\nexport LS_COLORS
		fmt.Printf("LS_COLORS='%s';\nexport LS_COLORS\n", value)
	}
}

// buildLSColors constructs the colon-separated LS_COLORS value from entries.
func buildLSColors(entries []dbEntry) string {
	if len(entries) == 0 {
		return ""
	}
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = e.key + "=" + e.value
	}
	return strings.Join(parts, ":") + ":"
}

func printUsage() {
	fmt.Fprintf(os.Stderr,
		"Usage: %s [OPTION]... [FILE]\nOutput commands to set LS_COLORS.\n"+
			"\n  -b, --sh, --bourne-shell    output Bourne shell code\n"+
			"  -c, --csh, --c-shell        output C shell code\n"+
			"  -p, --print-database        output defaults\n"+
			"      --help                  display this help\n"+
			"      --version               output version information\n",
		programName)
}
