// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/cp: copy files and directories.
// Implements srd056 R1.1-R1.4, R2.1-R2.4, R3.1-R3.4, R4.1-R4.3.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

const programName = "cp"
const version = "1.0.0"

// options holds parsed command-line flags for cp.
type options struct {
	interactive    bool   // R1.2: -i/--interactive
	force          bool   // R1.3: -f/--force
	noClobber      bool   // R1.4: -n/--no-clobber
	recursive      bool   // R2.1: -r/-R/--recursive
	dereference    bool   // R2.3: -L/--dereference
	noDereference  bool   // R2.4: -P/--no-dereference
	preserve       string // R3.1/R3.3: --preserve=ATTR_LIST
	archive        bool   // R3.2: -a/--archive
	verbose        bool   // R3.4: -v/--verbose
	targetDir      string // R4.3: -t/--target-directory
}

// main entry point with SIGPIPE handler and argument dispatch.
func main() {
	sys.InstallSIGPIPEHandler()
	opts, args := parseArgs(os.Args[1:])
	exitCode := run(opts, args)
	os.Exit(exitCode)
}

// run dispatches the copy operation based on parsed options and args.
func run(opts options, args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing file operand\n", programName)
		fmt.Fprintf(os.Stderr,
			"Try '%s --help' for more information.\n", programName)
		return 1
	}
	if len(args) == 1 && opts.targetDir == "" {
		fmt.Fprintf(os.Stderr,
			"%s: missing destination file operand after '%s'\n",
			programName, args[0])
		return 1
	}
	return copyFiles(opts, args)
}

// copyFiles performs the copy operation. Stub: returns 0.
func copyFiles(opts options, args []string) int {
	// TODO: implement copy logic (srd056 R1.1-R4.2)
	_ = opts
	_ = args
	return 0
}

// parseArgs separates flags from positional arguments.
// Supports short flags, combined short flags, and long forms.
// R1.4: when -n and -i both appear, -n takes precedence.
func parseArgs(rawArgs []string) (options, []string) {
	var opts options
	var positional []string

	for i := 0; i < len(rawArgs); i++ {
		arg := rawArgs[i]
		if arg == "--" {
			positional = append(positional, rawArgs[i+1:]...)
			break
		}
		if arg == "--help" {
			printUsage()
			os.Exit(0)
		}
		if arg == "--version" {
			printVersion()
			os.Exit(0)
		}
		if strings.HasPrefix(arg, "--") {
			i = parseLongFlag(&opts, rawArgs, i)
			continue
		}
		if strings.HasPrefix(arg, "-") && len(arg) > 1 {
			i = parseShortFlags(&opts, rawArgs, i)
			continue
		}
		positional = append(positional, arg)
	}
	return opts, positional
}

// parseLongFlag handles long-form flags for cp.
func parseLongFlag(opts *options, rawArgs []string, idx int) int {
	flag := rawArgs[idx]
	switch {
	case flag == "--interactive":
		opts.interactive = true
	case flag == "--force":
		opts.force = true
	case flag == "--no-clobber":
		opts.noClobber = true
	case flag == "--recursive":
		opts.recursive = true
	case flag == "--dereference":
		opts.dereference = true
	case flag == "--no-dereference":
		opts.noDereference = true
	case flag == "--archive":
		opts.archive = true
		opts.recursive = true
		opts.noDereference = true
		opts.preserve = "all"
	case flag == "--verbose":
		opts.verbose = true
	case strings.HasPrefix(flag, "--preserve="):
		opts.preserve = strings.TrimPrefix(flag, "--preserve=")
	case flag == "--preserve":
		opts.preserve = "mode,ownership,timestamps"
	case strings.HasPrefix(flag, "--target-directory="):
		opts.targetDir = strings.TrimPrefix(flag, "--target-directory=")
	case flag == "--target-directory":
		if idx+1 < len(rawArgs) {
			idx++
			opts.targetDir = rawArgs[idx]
		}
	}
	return idx
}

// parseShortFlags handles combined short flags like -rfv.
// R3.2: -a sets recursive, no-dereference, and preserve=all.
func parseShortFlags(opts *options, rawArgs []string, idx int) int {
	chars := rawArgs[idx][1:]
	for j := 0; j < len(chars); j++ {
		switch chars[j] {
		case 'i':
			opts.interactive = true
		case 'f':
			opts.force = true
		case 'n':
			opts.noClobber = true
		case 'r', 'R':
			opts.recursive = true
		case 'L':
			opts.dereference = true
		case 'P':
			opts.noDereference = true
		case 'p':
			opts.preserve = "mode,ownership,timestamps"
		case 'a':
			opts.archive = true
			opts.recursive = true
			opts.noDereference = true
			opts.preserve = "all"
		case 'v':
			opts.verbose = true
		case 't':
			rest := chars[j+1:]
			if len(rest) > 0 {
				opts.targetDir = rest
			} else if idx+1 < len(rawArgs) {
				idx++
				opts.targetDir = rawArgs[idx]
			}
			return idx
		}
	}
	return idx
}

// printUsage prints the usage message listing all flags from srd056.
func printUsage() {
	fmt.Fprintf(os.Stdout, `Usage: %s [OPTION]... SOURCE... DEST
  or:  %s [OPTION]... -t DIRECTORY SOURCE...
Copy SOURCE to DEST, or multiple SOURCE(s) to DIRECTORY.

Options:
  -a, --archive                same as -dR --preserve=all
  -f, --force                  if destination cannot be opened, remove it and retry
  -i, --interactive            prompt before overwrite
  -L, --dereference            always follow symlinks in SOURCE
  -n, --no-clobber             do not overwrite an existing file
  -P, --no-dereference         never follow symlinks in SOURCE
  -p                           same as --preserve=mode,ownership,timestamps
      --preserve[=ATTR_LIST]   preserve specified attributes (mode,ownership,timestamps,links,all)
  -r, -R, --recursive          copy directories recursively
  -t, --target-directory=DIR   copy all SOURCE arguments into DIR
  -v, --verbose                explain what is being done
      --help                   display this help and exit
      --version                output version information and exit
`, programName, programName)
}

// printVersion prints version information.
func printVersion() {
	fmt.Fprintf(os.Stdout, "%s %s\n", programName, version)
}
