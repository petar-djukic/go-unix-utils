// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd040-printenv: Print Environment Variables.
// Covers R1.1-R1.3 (default behavior), R2.1-R2.4 (output formatting and exit codes),
// R3.1-R3.3 (differential testing and edge cases).
package main

import (
	"fmt"
	"os"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const progName = "printenv"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and executes the printenv operation. Returns exit code.
func run(args []string) int {
	cfg, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		printTryHelp()
		return 2
	}
	if cfg.showHelp {
		return printHelp()
	}
	if cfg.showVersion {
		return printVersion()
	}
	if len(cfg.variables) == 0 {
		return printAllVars(cfg.nullDelim)
	}
	return printNamedVars(cfg.variables, cfg.nullDelim)
}

// config holds parsed command-line options and arguments.
type config struct {
	nullDelim   bool
	showHelp    bool
	showVersion bool
	variables   []string
}

// parseArgs processes command-line arguments into a config.
// R2.1: recognizes -0 and --null flags.
func parseArgs(args []string) (config, error) {
	var cfg config
	i := 0
	for ; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			i++
			cfg.variables = append(cfg.variables, args[i:]...)
			return cfg, nil
		case arg == "-0" || arg == "--null":
			// R2.1: NUL-delimited output.
			cfg.nullDelim = true
		case arg == "--help":
			cfg.showHelp = true
			return cfg, nil
		case arg == "--version":
			cfg.showVersion = true
			return cfg, nil
		case len(arg) > 1 && arg[0] == '-' && arg[1] != '-':
			if err := parseShortFlags(&cfg, arg); err != nil {
				return cfg, err
			}
		case len(arg) > 2 && arg[0] == '-' && arg[1] == '-':
			return cfg, fmt.Errorf("unrecognized option '%s'", arg)
		default:
			cfg.variables = append(cfg.variables, args[i:]...)
			return cfg, nil
		}
	}
	return cfg, nil
}

// parseShortFlags processes combined short flags in a single argument.
func parseShortFlags(cfg *config, arg string) error {
	for j := 1; j < len(arg); j++ {
		switch arg[j] {
		case '0':
			cfg.nullDelim = true
		default:
			return fmt.Errorf("invalid option -- '%c'", arg[j])
		}
	}
	return nil
}

// printAllVars prints every environment variable in NAME=VALUE format.
// R1.1: prints all variables, one per line, exits 0.
// R2.4: always exits 0 when no VARIABLE arguments given.
func printAllVars(nullDelim bool) int {
	delim := "\n"
	if nullDelim {
		delim = "\x00"
	}
	for _, e := range os.Environ() {
		if _, err := fmt.Fprint(os.Stdout, e+delim); err != nil {
			return 2
		}
	}
	return 0
}

// printNamedVars prints the value of each named variable.
// R1.2: prints only the value, not the name or '='.
// R1.3: produces no output for missing variables, no error message.
// R2.2: exits 0 if all found.
// R2.3: exits 1 if any not found.
func printNamedVars(vars []string, nullDelim bool) int {
	delim := "\n"
	if nullDelim {
		delim = "\x00"
	}
	exitCode := 0
	for _, name := range vars {
		val, ok := os.LookupEnv(name)
		if !ok {
			exitCode = 1
			continue
		}
		if _, err := fmt.Fprint(os.Stdout, val+delim); err != nil {
			return 2
		}
	}
	return exitCode
}

// printTryHelp writes the "Try --help" hint to stderr.
func printTryHelp() {
	fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
}

// printHelp writes usage information to stdout and returns the exit code.
// R2.2: --help prints usage to stdout and exits 0.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: printenv [OPTION]... [VARIABLE]...
Print the values of the specified environment VARIABLE(s).
If no VARIABLE is specified, print name and value pairs for them all.

  -0, --null     end each output line with NUL, not newline
      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 2
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
// R2.3: --version prints version to stdout and exits 0.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "%s (go-unix-utils) %s\n", progName, version)
	if err != nil {
		return 2
	}
	return 0
}
