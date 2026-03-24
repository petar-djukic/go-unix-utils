// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd052-tty: Print Terminal File Name.
// Covers R1.1-R1.3 (default behavior, not-a-tty, silent mode),
// R2.1-R2.2 (extra operand/unknown flag errors),
// R3.1 (differential testing).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run parses arguments and prints the terminal name. Returns exit code.
// R1.1: prints tty name when stdin is a terminal.
// R1.2: prints "not a tty" and exits 1 when stdin is not a terminal.
func run(args []string) int {
	silent, exitCode := parseArgs(args)
	if exitCode >= 0 {
		return exitCode
	}
	isTTY := sys.IsTerminal(os.Stdin.Fd())
	if !silent {
		output := "not a tty"
		if isTTY {
			if name, err := ttyName(int(os.Stdin.Fd())); err == nil {
				output = name
			}
		}
		if _, err := fmt.Println(output); err != nil {
			return 1
		}
	}
	if isTTY {
		return 0
	}
	return 1
}

// parseArgs processes command-line arguments. Returns the silent flag and
// an exit code. An exit code of -1 means parsing succeeded and execution
// should continue.
func parseArgs(args []string) (bool, int) {
	silent := false
	endOfOpts := false
	for _, arg := range args {
		if endOfOpts {
			return false, printExtraOperand(arg)
		}
		if arg == "--" {
			endOfOpts = true
			continue
		}
		if len(arg) > 1 && arg[0] == '-' && arg[1] != '-' {
			if code := parseShortFlags(arg[1:], &silent); code >= 0 {
				return false, code
			}
			continue
		}
		switch arg {
		case "--help":
			return false, printHelp()
		case "--version":
			return false, printVersion()
		case "--silent", "--quiet":
			silent = true
		default:
			if len(arg) > 1 && arg[0] == '-' {
				return false, printUnknownFlag(arg)
			}
			return false, printExtraOperand(arg)
		}
	}
	return silent, -1
}

// parseShortFlags processes a cluster of short flags. Returns -1 on success
// or an exit code on error. R1.3: -s sets silent mode.
func parseShortFlags(flags string, silent *bool) int {
	for _, c := range flags {
		if c == 's' {
			*silent = true
		} else {
			// R2.2: unknown flag exits 2.
			fmt.Fprintf(os.Stderr, "tty: invalid option -- '%c'\n", c)
			fmt.Fprintln(os.Stderr, "Try 'tty --help' for more information.")
			return 2
		}
	}
	return -1
}

// ttyName returns the terminal device name for the given file descriptor.
func ttyName(fd int) (string, error) {
	// Linux: /proc/self/fd/N is a symlink to the device path.
	name, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
	if err == nil {
		return name, nil
	}
	var st syscall.Stat_t
	if err := syscall.Fstat(fd, &st); err != nil {
		return "", fmt.Errorf("fstat: %w", err)
	}
	// Check common tty device paths by glob pattern.
	for _, pattern := range []string{"/dev/pts/*", "/dev/ttys*"} {
		if match, err := matchGlob(pattern, st); err == nil {
			return match, nil
		}
	}
	return scanDev(st)
}

// matchGlob searches files matching pattern for a device with matching rdev.
func matchGlob(pattern string, target syscall.Stat_t) (string, error) {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	for _, path := range matches {
		var st syscall.Stat_t
		if syscall.Stat(path, &st) == nil && st.Rdev == target.Rdev {
			return path, nil
		}
	}
	return "", fmt.Errorf("no match for pattern %s", pattern)
}

// scanDev searches /dev for a character device matching the target rdev.
func scanDev(target syscall.Stat_t) (string, error) {
	entries, err := os.ReadDir("/dev")
	if err != nil {
		return "", fmt.Errorf("reading /dev: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := "/dev/" + e.Name()
		var st syscall.Stat_t
		if syscall.Stat(path, &st) == nil && st.Rdev == target.Rdev {
			return path, nil
		}
	}
	return "", fmt.Errorf("tty name not found")
}

// printHelp writes usage information to stdout and returns the exit code.
// R2.1: --help outputs usage and exits 0.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: tty [OPTION]...
Print the file name of the terminal connected to standard input.

  -s, --silent, --quiet   print nothing, only return an exit status
      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
// R2.2: --version outputs version and exits 0.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "tty (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}

// printUnknownFlag writes an error for an unrecognized option and returns
// exit code 2. R2.2: matches GNU tty error format.
func printUnknownFlag(flag string) int {
	fmt.Fprintf(os.Stderr, "tty: unrecognized option '%s'\n", flag)
	fmt.Fprintln(os.Stderr, "Try 'tty --help' for more information.")
	return 2
}

// printExtraOperand writes an error for an extra operand and returns
// exit code 2. R2.1: matches GNU tty error format.
func printExtraOperand(arg string) int {
	fmt.Fprintf(os.Stderr, "tty: extra operand '%s'\n", arg)
	fmt.Fprintln(os.Stderr, "Try 'tty --help' for more information.")
	return 2
}
