// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements: prd049-realpath R1.1–R1.4
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error and --help output.
const programName = "realpath"

func main() {
	// D2: install SIGPIPE handler before any I/O.
	sys.InstallSIGPIPEHandler()

	args := os.Args[1:]

	var operands []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--help":
			printHelp()
			return
		case arg == "--version":
			printVersion()
			return
		case arg == "--":
			// End of flags; remaining args are operands.
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			// R1.4/R3.2: unrecognized long option.
			fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
			fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
			os.Exit(1)
		case strings.HasPrefix(arg, "-") && len(arg) > 1:
			// Short flag — no short flags supported in this scope.
			cluster := arg[1:]
			for _, ch := range cluster {
				fmt.Fprintf(os.Stderr, "%s: invalid option -- '%c'\n", programName, ch)
				fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
				os.Exit(1)
			}
		default:
			operands = append(operands, arg)
		}
	}

	// R3.1: no operands is a usage error.
	if len(operands) == 0 {
		fmt.Fprintf(os.Stderr, "%s: missing operand\n", programName)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", programName)
		os.Exit(1)
	}

	// R1.1, R1.2: resolve each path and print one per line.
	// R1.3: exit 1 if any path fails, but continue processing all paths.
	exitCode := 0
	for _, path := range operands {
		resolved, err := resolvePath(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s: No such file or directory\n", programName, path)
			exitCode = 1
			continue
		}
		fmt.Println(resolved)
	}
	os.Exit(exitCode)
}

// resolvePath resolves a path to its canonical absolute form with all symlinks resolved.
// GNU realpath default mode requires all but the last component to exist. If the full
// path exists, resolve it directly. Otherwise, resolve the parent directory and append
// the final component.
//
// R1.1: resolve symlinks and produce absolute canonical path.
func resolvePath(path string) (string, error) {
	// Try resolving the full path first.
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return filepath.Abs(resolved)
	}

	// Default mode: all but the last component must exist.
	// Resolve the parent directory and append the base name.
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	resolvedDir, dirErr := filepath.EvalSymlinks(dir)
	if dirErr != nil {
		return "", dirErr
	}

	absDir, dirErr := filepath.Abs(resolvedDir)
	if dirErr != nil {
		return "", dirErr
	}

	return filepath.Join(absDir, base), nil
}

// printHelp writes usage information to stdout and exits 0.
func printHelp() {
	fmt.Print(`Usage: realpath [OPTION]... FILE...
Print the resolved absolute file name;
all but the last component must exist

      --help     display this help and exit
      --version  output version information and exit
`)
}

// printVersion writes version information to stdout and exits 0.
func printVersion() {
	fmt.Println("realpath (go-unix-utils) 0.1")
}
