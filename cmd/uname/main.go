// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Implements prd044-uname R1.1 (default prints kernel name),
// R1.2 (-s prints kernel name), R1.3 (-n prints node hostname),
// R1.4 (-r prints kernel release), R1.5 (-v prints kernel version),
// R1.6 (-m prints machine hardware name), R1.7 (-p prints processor type),
// R1.8 (-i prints hardware platform), R1.9 (-o prints operating system name),
// R2.1 (-a prints all fields), R2.2 (combined flags print in canonical order).
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
	fieldMachine                     // -m: machine hardware name
	fieldProcessor                   // -p: processor type
	fieldPlatform                    // -i: hardware platform
	fieldOperating                   // -o: operating system
)

// totalFields is the number of fields currently implemented.
const totalFields = 8

func main() {
	sys.InstallSIGPIPEHandler()
	os.Exit(run(os.Args[1:]))
}

// parsedArgs holds the result of argument parsing.
type parsedArgs struct {
	selected [totalFields]bool
	allMode  bool // true when -a was used; suppresses "unknown" values
}

// run processes arguments and prints the requested uname fields.
// Returns the exit code.
func run(args []string) int {
	parsed, err := parseArgs(args)
	if err != nil {
		printError(err.Error())
		return 1
	}
	return printFields(parsed)
}

// parseArgs parses command-line arguments and returns which fields
// are selected. If no flags are given, selects kernel name (R1.1).
func parseArgs(args []string) (parsedArgs, error) {
	var result parsedArgs
	hasFlag := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--help" || arg == "--version" {
			fmt.Print(helpTextFor(arg))
			os.Exit(0)
		}
		if !strings.HasPrefix(arg, "-") {
			return result, fmt.Errorf("extra operand '%s'", arg)
		}
		if strings.HasPrefix(arg, "--") {
			return result, fmt.Errorf("unrecognized option '%s'", arg)
		}
		if err := parseShortFlags(arg[1:], &result); err != nil {
			return result, err
		}
		hasFlag = true
	}
	// R1.1: default (no flags) prints kernel name.
	if !hasFlag {
		result.selected[fieldSysname] = true
	}
	return result, nil
}

// parseShortFlags processes a string of short flag characters.
func parseShortFlags(flags string, result *parsedArgs) error {
	for _, ch := range flags {
		if ch == 'a' {
			selectAll(&result.selected)
			result.allMode = true
			continue
		}
		idx, ok := flagToField(ch)
		if !ok {
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
		result.selected[idx] = true
	}
	return nil
}

// selectAll marks all fields as selected (R2.1).
func selectAll(selected *[totalFields]bool) {
	for i := range totalFields {
		selected[i] = true
	}
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
	case 'm':
		return fieldMachine, true
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
// R2.1: when allMode is true, "unknown" values are omitted.
func printFields(parsed parsedArgs) int {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot get system information: %v\n",
			programName, err)
		return 1
	}
	values := fieldValues(&utsname)
	var parts []string
	for i := range totalFields {
		if !parsed.selected[i] {
			continue
		}
		if parsed.allMode && values[i] == "unknown" {
			continue
		}
		parts = append(parts, values[i])
	}
	fmt.Println(strings.Join(parts, " "))
	return 0
}

// fieldValues extracts the string value for each field from utsname.
func fieldValues(u *unix.Utsname) [totalFields]string {
	machine := unix.ByteSliceToString(u.Machine[:])
	return [totalFields]string{
		fieldSysname:   unix.ByteSliceToString(u.Sysname[:]),
		fieldNodename:  unix.ByteSliceToString(u.Nodename[:]),
		fieldRelease:   unix.ByteSliceToString(u.Release[:]),
		fieldVersion:   unix.ByteSliceToString(u.Version[:]),
		fieldMachine:   machine,
		fieldProcessor: processorType(machine),
		fieldPlatform:  hardwarePlatform(),
		fieldOperating: operatingSystem(),
	}
}

// processorType returns the processor type based on the machine
// hardware name. On Darwin, guname maps arm64 to "arm". On Linux and
// other platforms, guname returns "unknown" (R1.7).
func processorType(machine string) string {
	if runtime.GOOS == "darwin" {
		return darwinProcessorType(machine)
	}
	return "unknown"
}

// darwinProcessorType maps Darwin machine names to processor families.
func darwinProcessorType(machine string) string {
	switch machine {
	case "arm64":
		return "arm"
	case "x86_64":
		return "x86_64"
	default:
		return "unknown"
	}
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

  -a    print all information in the following order
  -s    print the kernel name
  -n    print the network node hostname
  -r    print the kernel release
  -v    print the kernel version
  -m    print the machine hardware name
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
