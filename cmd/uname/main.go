// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd044-uname: Print System Information.
// Covers R1.1-R1.9 (default/no-arg, -s, -n, -r, -v, -m, -p, -i, -o flags),
// R2.1 (-a combined output), R2.2 (multi-flag canonical order),
// R3.1-R3.2 (error handling for invalid options and operands),
// R4.1-R4.2 (--help, --version, exit codes).
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

const unknownField = "unknown"

// fieldIndex enumerates the canonical field positions for -a output order.
// R2.2: kernel name, node name, release, version, machine, processor, platform, OS.
type fieldIndex int

const (
	fieldSysname  fieldIndex = iota // -s: kernel name
	fieldNodename                   // -n: network node hostname
	fieldRelease                    // -r: kernel release
	fieldVersion                    // -v: kernel version
	fieldMachine                    // -m: machine hardware name
	fieldProcessor                  // -p: processor type
	fieldHardware                   // -i: hardware platform
	fieldOS                         // -o: operating system name
	fieldCount
)

// parseResult holds the parsed flag selections.
type parseResult struct {
	selected []bool
	allFlag  bool // true when -a was specified
}

// osName returns the operating system name matching GNU uname output.
func osName() string {
	switch runtime.GOOS {
	case "linux":
		return "GNU/Linux"
	case "darwin":
		return "Darwin"
	default:
		return runtime.GOOS
	}
}

func main() {
	sys.InstallSIGPIPEHandler()

	exitCode := run(os.Args[1:])
	os.Exit(exitCode)
}

// run parses arguments and prints selected system information fields.
func run(args []string) int {
	result, code := parseArgs(args)
	if code >= 0 {
		return code
	}

	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "uname: %v\n", err)
		return 1
	}

	fields := extractFields(&utsname)
	return printFields(fields, result)
}

// parseArgs processes flags and returns the parse result.
// Returns (result, -1) on success, or (zero, exitCode) on early exit.
func parseArgs(args []string) (parseResult, int) {
	var result parseResult
	result.selected = make([]bool, fieldCount)
	anySelected := false

	for _, arg := range args {
		if arg == "--help" {
			return result, printHelp()
		}
		if arg == "--version" {
			return result, printVersion()
		}
		if arg == "--" {
			continue
		}
		// R3.2: unrecognized long options.
		if strings.HasPrefix(arg, "--") {
			fmt.Fprintf(os.Stderr, "uname: unrecognized option '%s'\n", arg)
			fmt.Fprintln(os.Stderr, "Try 'uname --help' for more information.")
			return result, 1
		}
		if len(arg) > 1 && arg[0] == '-' {
			code := parseShortFlags(arg[1:], &result)
			if code >= 0 {
				return result, code
			}
			anySelected = true
			continue
		}
		// Positional operand: error per R3.1.
		fmt.Fprintf(os.Stderr, "uname: extra operand '%s'\n", arg)
		fmt.Fprintln(os.Stderr, "Try 'uname --help' for more information.")
		return result, 1
	}

	// R1.1: no arguments defaults to -s (kernel name).
	if !anySelected {
		result.selected[fieldSysname] = true
	}
	return result, -1
}

// parseShortFlags processes a short flag string (without leading '-').
// Returns -1 on success, or a non-negative exit code on error.
func parseShortFlags(flags string, result *parseResult) int {
	for _, ch := range flags {
		switch ch {
		case 'a':
			// R2.1: select all fields.
			result.allFlag = true
			for i := range result.selected {
				result.selected[i] = true
			}
		case 's':
			result.selected[fieldSysname] = true
		case 'n':
			result.selected[fieldNodename] = true
		case 'r':
			result.selected[fieldRelease] = true
		case 'v':
			result.selected[fieldVersion] = true
		case 'm':
			result.selected[fieldMachine] = true
		case 'p':
			// R1.7: processor type.
			result.selected[fieldProcessor] = true
		case 'i':
			// R1.8: hardware platform.
			result.selected[fieldHardware] = true
		case 'o':
			// R1.9: operating system name.
			result.selected[fieldOS] = true
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
	machine := utsToString(u.Machine)
	fields := make([]string, fieldCount)
	fields[fieldSysname] = utsToString(u.Sysname)
	fields[fieldNodename] = utsToString(u.Nodename)
	fields[fieldRelease] = utsToString(u.Release)
	fields[fieldVersion] = utsToString(u.Version)
	fields[fieldMachine] = machine
	// R1.7: processor type — derived from machine on Darwin, "unknown" otherwise.
	fields[fieldProcessor] = processorType(machine)
	// R1.8: hardware platform — "unknown" when not determinable.
	fields[fieldHardware] = unknownField
	// R1.9: operating system name.
	fields[fieldOS] = osName()
	return fields
}

// processorType returns the processor type for -p output.
// On Darwin, maps machine architecture to CPU family (e.g., arm64 → arm).
// On other platforms, returns "unknown" matching GNU coreutils behavior.
func processorType(machine string) string {
	if runtime.GOOS != "darwin" {
		return unknownField
	}
	return darwinProcessorFamily(machine)
}

// darwinProcessorFamily maps a Darwin machine name to its CPU family.
func darwinProcessorFamily(machine string) string {
	if strings.HasPrefix(machine, "arm") {
		return "arm"
	}
	if strings.HasPrefix(machine, "x86") || machine == "i386" || machine == "i686" {
		return "i386"
	}
	return machine
}

// printFields outputs selected fields space-separated with trailing newline.
// R2.2: fields are printed in canonical order determined by fieldIndex.
// When -a is used, "unknown" fields are omitted (matching GNU uname behavior).
func printFields(fields []string, result parseResult) int {
	var parts []string
	for i, sel := range result.selected {
		if !sel {
			continue
		}
		// GNU uname -a omits "unknown" processor and hardware platform fields.
		if result.allFlag && fields[i] == unknownField {
			continue
		}
		parts = append(parts, fields[i])
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
  -p             print the processor type
  -i             print the hardware platform
  -o             print the operating system
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
