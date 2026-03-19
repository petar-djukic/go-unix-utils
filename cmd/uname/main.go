// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd044-uname R1.1 (default prints kernel name),
// R1.2 (-s prints kernel name), R1.3 (-n prints node hostname),
// R1.4 (-r prints kernel release), R1.5 (-v prints kernel version),
// R1.7 (-p prints processor type), R1.8 (-i prints hardware platform),
// R1.9 (-o prints operating system name).
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is used in error messages.
const programName = "uname"

// fieldIndex defines the canonical ordering of uname fields.
type fieldIndex int

const (
	fieldSysname   fieldIndex = iota // -s: kernel name
	fieldNodename                    // -n: node hostname
	fieldRelease                     // -r: kernel release
	fieldVersion                     // -v: kernel version
	fieldProcessor                   // -p: processor type
	fieldPlatform                    // -i: hardware platform
	fieldOperating                   // -o: operating system
)

// totalFields is the number of fields currently implemented.
const totalFields = 7

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// run processes arguments and prints the requested uname fields.
// Returns the exit code.
func run(args []string) int {
	selected, err := parseArgs(args)
	if err != nil {
		printError(err.Error())
		return 1
	}
	return printFields(selected)
}

// parseArgs parses command-line arguments and returns which fields
// are selected. If no flags are given, selects kernel name (R1.1).
func parseArgs(args []string) ([totalFields]bool, error) {
	var selected [totalFields]bool
	hasFlag := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--help" || arg == "--version" {
			// Not in R1.1-R1.4 scope; treat like GNU does.
			fmt.Print(helpTextFor(arg))
			os.Exit(0)
		}
		if !strings.HasPrefix(arg, "-") {
			return selected, fmt.Errorf("extra operand '%s'", arg)
		}
		if err := parseShortFlags(arg[1:], &selected); err != nil {
			return selected, err
		}
		hasFlag = true
	}
	// R1.1: default (no flags) prints kernel name.
	if !hasFlag {
		selected[fieldSysname] = true
	}
	return selected, nil
}

// parseShortFlags processes a string of short flag characters.
func parseShortFlags(flags string, selected *[totalFields]bool) error {
	for _, ch := range flags {
		idx, ok := flagToField(ch)
		if !ok {
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
		selected[idx] = true
	}
	return nil
}

// flagToField maps a flag character to its field index.
func flagToField(ch rune) (fieldIndex, bool) {
	switch ch {
	case 's':
		return fieldSysname, true
	case 'n':
		return fieldNodename, true
	case 'r':
		return fieldRelease, true
	case 'v':
		return fieldVersion, true
	case 'p':
		return fieldProcessor, true
	case 'i':
		return fieldPlatform, true
	case 'o':
		return fieldOperating, true
	default:
		return 0, false
	}
}

// printFields retrieves uname info and prints the selected fields
// in canonical order, space-separated, followed by a newline.
func printFields(selected [totalFields]bool) int {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot get system information: %v\n",
			programName, err)
		return 1
	}
	values := fieldValues(&utsname)
	var parts []string
	for i := range totalFields {
		if selected[i] {
			parts = append(parts, values[i])
		}
	}
	fmt.Println(strings.Join(parts, " "))
	return 0
}

// fieldValues extracts the string value for each field from utsname.
func fieldValues(u *unix.Utsname) [totalFields]string {
	return [totalFields]string{
		fieldSysname:   unix.ByteSliceToString(u.Sysname[:]),
		fieldNodename:  unix.ByteSliceToString(u.Nodename[:]),
		fieldRelease:   unix.ByteSliceToString(u.Release[:]),
		fieldVersion:   unix.ByteSliceToString(u.Version[:]),
		fieldProcessor: processorType(),
		fieldPlatform:  hardwarePlatform(),
		fieldOperating: operatingSystem(),
	}
}

// processorType returns the processor type. GNU guname returns
// "unknown" on Darwin and most Linux configurations (R1.7).
func processorType() string {
	return "unknown"
}

// hardwarePlatform returns the hardware platform. GNU guname returns
// "unknown" on Darwin and most Linux configurations (R1.8).
func hardwarePlatform() string {
	return "unknown"
}

// operatingSystem returns the operating system name.
// R1.9: GNU guname returns "GNU/Linux" on Linux and "Darwin" on macOS.
func operatingSystem() string {
	if runtime.GOOS == "linux" {
		return "GNU/Linux"
	}
	return "Darwin"
}

// helpTextFor returns text for --help or --version.
func helpTextFor(flag string) string {
	if flag == "--version" {
		return "uname (go-unix-utils) 1.0\n"
	}
	return `Usage: uname [OPTION]...
Print certain system information.  With no OPTION, same as -s.

  -s    print the kernel name
  -n    print the network node hostname
  -r    print the kernel release
  -v    print the kernel version
  -p    print the processor type
  -i    print the hardware platform
  -o    print the operating system
      --help     display this help and exit
      --version  output version information and exit
`
}

// printError writes a formatted error message to stderr.
func printError(msg string) {
	fmt.Fprintf(os.Stderr,
		"%s: %s\nTry '%s --help' for more information.\n",
		programName, msg, programName)
}
