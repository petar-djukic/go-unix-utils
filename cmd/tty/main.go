// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/tty: print terminal file name.
// Implements srd052-tty R1.1, R1.2, R1.3, R2.1.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "tty"

// helpText is the usage message printed when --help is passed.
// R2.1: --help prints usage to stdout and exits 0.
const helpText = `Usage: tty [OPTION]...
Print the file name of the terminal connected to standard input.

  -s, --silent, --quiet   print nothing, only return an exit status
      --help     display this help and exit
      --version  output version information and exit
`

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils)"

func main() {
	sys.InstallSIGPIPEHandler()

	silent, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(2)
	}

	os.Exit(runTTY(silent))
}

// runTTY checks whether stdin is a terminal and prints the device name.
// R1.1: terminal → print device path, exit 0.
// R1.2: not terminal → print "not a tty", exit 1.
// R1.3: silent → suppress output, exit code only.
func runTTY(silent bool) int {
	if !sys.IsTerminal(os.Stdin.Fd()) {
		if !silent {
			fmt.Println("not a tty")
		}
		return 1
	}
	if !silent {
		name, err := ttyName()
		if err != nil {
			fmt.Println("not a tty")
			return 1
		}
		fmt.Println(name)
	}
	return 0
}

// ttyName returns the terminal device path for stdin.
// Uses /dev/fd/0 on Darwin and /proc/self/fd/0 on Linux.
func ttyName() (string, error) {
	path := stdinFDPath()
	target, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("reading terminal name: %w", err)
	}
	return target, nil
}

// stdinFDPath returns the platform-specific path to the stdin fd symlink.
func stdinFDPath() string {
	if runtime.GOOS == "linux" {
		return "/proc/self/fd/0"
	}
	return "/dev/fd/0"
}

// parseArgs processes command-line arguments and returns the silent flag.
// R1.3: -s, --silent, --quiet set silent mode.
// R2.1: extra operands produce an error with exit 2.
// R2.2: unknown flags produce an error with exit 2.
func parseArgs(args []string) (bool, error) {
	silent := false
	for _, arg := range args {
		switch arg {
		case "--help":
			fmt.Print(helpText)
			os.Exit(0)
		case "--version":
			fmt.Println(versionText)
			os.Exit(0)
		case "-s", "--silent", "--quiet":
			silent = true
		default:
			if strings.HasPrefix(arg, "-") && arg != "-" {
				return false, fmt.Errorf("unrecognized option '%s'", arg)
			}
			return false, fmt.Errorf("extra operand '%s'", arg)
		}
	}
	return silent, nil
}
