// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd044-uname: Print System Information.
// Covers R1.1-R1.5 (default/no-arg, -s, -n, -r, -v flags),
// R2.1 (-a combined output in canonical order).
package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

// fieldIndex enumerates the canonical field positions for -a output order.
type fieldIndex int

const (
	fieldSysname  fieldIndex = iota // -s: kernel name
	fieldNodename                   // -n: network node hostname
	fieldRelease                    // -r: kernel release
	fieldVersion                    // -v: kernel version
	fieldMachine                    // -m: machine hardware name
	fieldCount
)

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run parses arguments and prints selected system information fields.
func run(args []string) int {
	selected, code := parseArgs(args)
	if code >= 0 {
		return code
	}

	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "uname: %v\n", err)
		return 1
	}

	fields := extractFields(&utsname)
	return printFields(fields, selected)
}

// parseArgs processes flags and returns the selected field mask.
// Returns (selected, -1) on success, or (nil, exitCode) on early exit.
func parseArgs(args []string) ([]bool, int) {
	selected := make([]bool, fieldCount)
	anySelected := false

	for _, arg := range args {
		if arg == "--help" {
			return nil, printHelp()
		}
		if arg == "--version" {
			return nil, printVersion()
		}
		if arg == "--" {
			continue
		}
		if len(arg) > 1 && arg[0] == '-' {
			code := parseShortFlags(arg[1:], selected)
			if code >= 0 {
				return nil, code
			}
			anySelected = true
			continue
		}
		// Positional operand: error per R3.1.
		fmt.Fprintf(os.Stderr, "uname: extra operand '%s'\n", arg)
		fmt.Fprintln(os.Stderr, "Try 'uname --help' for more information.")
		return nil, 1
	}

	// R1.1: no arguments defaults to -s (kernel name).
	if !anySelected {
		selected[fieldSysname] = true
	}
	return selected, -1
}

// parseShortFlags processes a short flag string (without leading '-').
// Returns -1 on success, or a non-negative exit code on error.
func parseShortFlags(flags string, selected []bool) int {
	for _, ch := range flags {
		switch ch {
		case 'a':
			// R2.1: select all fields.
			for i := range selected {
				selected[i] = true
			}
		case 's':
			selected[fieldSysname] = true
		case 'n':
			selected[fieldNodename] = true
		case 'r':
			selected[fieldRelease] = true
		case 'v':
			selected[fieldVersion] = true
		case 'm':
			selected[fieldMachine] = true
		default:
			fmt.Fprintf(os.Stderr, "uname: invalid option -- '%c'\n", ch)
			fmt.Fprintln(os.Stderr, "Try 'uname --help' for more information.")
			return 1
		}
	}
	return -1
}

// extractFields reads all information fields from the utsname struct.
func extractFields(u *unix.Utsname) []string {
	fields := make([]string, fieldCount)
	fields[fieldSysname] = utsToString(u.Sysname)
	fields[fieldNodename] = utsToString(u.Nodename)
	fields[fieldRelease] = utsToString(u.Release)
	fields[fieldVersion] = utsToString(u.Version)
	fields[fieldMachine] = utsToString(u.Machine)
	return fields
}

// printFields outputs selected fields space-separated with trailing newline.
func printFields(fields []string, selected []bool) int {
	var parts []string
	for i, sel := range selected {
		if sel {
			parts = append(parts, fields[i])
		}
	}
	if _, err := fmt.Println(strings.Join(parts, " ")); err != nil {
		return 1
	}
	return 0
}

// utsToString converts a Utsname byte array field to a Go string.
func utsToString(field [256]byte) string {
	n := 0
	for n < len(field) && field[n] != 0 {
		n++
	}
	return string(field[:n])
}

// printHelp writes usage information to stdout and returns the exit code.
func printHelp() int {
	_, err := fmt.Fprint(os.Stdout, `Usage: uname [OPTION]...
Print certain system information.  With no OPTION, same as -s.

  -a             print all information, in the following order:
  -s             print the kernel name
  -n             print the network node hostname
  -r             print the kernel release
  -v             print the kernel version
  -m             print the machine hardware name
      --help     display this help and exit
      --version  output version information and exit
`)
	if err != nil {
		return 1
	}
	return 0
}

// printVersion writes version information to stdout and returns the exit code.
func printVersion() int {
	_, err := fmt.Fprintf(os.Stdout, "uname (go-unix-utils) %s\n", version)
	if err != nil {
		return 1
	}
	return 0
}
