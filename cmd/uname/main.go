// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// cmd/uname prints system information fields.
// Implements prd044-uname R1.1–R1.9, R2.1, R2.2, R3.1, R3.2.
package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// programName is the name used in error messages.
const programName = "uname"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys.InstallSIGPIPEHandler()

	var flags unameFlags
	anyFlag := false

	exit, done := parseFlags(os.Args[1:], &flags, &anyFlag)
	if done {
		os.Exit(exit)
	}

	// R1.1: no flags prints kernel name (equivalent to -s).
	if !anyFlag {
		flags.s = true
	}

	fields, err := collectFields(&flags)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", programName, err)
		os.Exit(1)
	}
	fmt.Println(strings.Join(fields, " "))
}

// unameFlags holds the state of each individual flag.
type unameFlags struct {
	s, n, r, v, m, p, i, o bool
	// allMode is true when -a was used; "unknown" fields are omitted.
	allMode bool
}

// setAll sets every flag to true, used by -a (R2.1).
func (f *unameFlags) setAll() {
	f.s = true
	f.n = true
	f.r = true
	f.v = true
	f.m = true
	f.p = true
	f.i = true
	f.o = true
}

// parseFlags processes argv for single-character flags and --version.
// Returns (exitCode, true) when the program should exit immediately.
func parseFlags(args []string, flags *unameFlags, anyFlag *bool) (int, bool) {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			fmt.Fprintf(os.Stderr, "%s: extra operand '%s'\n", programName, arg)
			return 1, true
		}
		if strings.HasPrefix(arg, "--") {
			return handleLongOption(arg)
		}
		if err := parseFlagChars(arg[1:], flags, anyFlag); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", programName, err)
			return 1, true
		}
	}
	return 0, false
}

// handleLongOption handles --help, --version and rejects unknown long options.
func handleLongOption(arg string) (int, bool) {
	// R3.2: --help prints usage information to stdout and exits 0.
	if arg == "--help" {
		printHelp()
		return 0, true
	}
	// --version prints version information and exits 0.
	if arg == "--version" {
		fmt.Printf("%s (go-unix-utils) %s\n", programName, version)
		return 0, true
	}
	fmt.Fprintf(os.Stderr, "%s: unrecognized option '%s'\n", programName, arg)
	return 1, true
}

// printHelp writes usage information to stdout (R3.2).
func printHelp() {
	fmt.Printf("Usage: %s [OPTION]...\n", programName)
	fmt.Println("Print certain system information.  With no OPTION, same as -s.")
	fmt.Println()
	fmt.Println("  -a             print all information, in the following order:")
	fmt.Println("  -s             print the kernel name")
	fmt.Println("  -n             print the network node hostname")
	fmt.Println("  -r             print the kernel release")
	fmt.Println("  -v             print the kernel version")
	fmt.Println("  -m             print the machine hardware name")
	fmt.Println("  -p             print the processor type or \"unknown\"")
	fmt.Println("  -i             print the hardware platform or \"unknown\"")
	fmt.Println("  -o             print the operating system")
	fmt.Println("      --help     display this help and exit")
	fmt.Println("      --version  output version information and exit")
}

// parseFlagChars processes each character in a flag group.
func parseFlagChars(chars string, flags *unameFlags, anyFlag *bool) error {
	for _, ch := range chars {
		switch ch {
		case 'a':
			flags.setAll()
			flags.allMode = true
		case 's':
			flags.s = true
		case 'n':
			flags.n = true
		case 'r':
			flags.r = true
		case 'v':
			flags.v = true
		case 'm':
			flags.m = true
		case 'p':
			flags.p = true
		case 'i':
			flags.i = true
		case 'o':
			flags.o = true
		default:
			return fmt.Errorf("invalid option -- '%c'", ch)
		}
		*anyFlag = true
	}
	return nil
}

// collectFields gathers the requested uname fields in canonical order.
// R2.2: fields are always emitted in canonical order regardless of
// the order flags were specified.
func collectFields(f *unameFlags) ([]string, error) {
	var fields []string
	// R1.2: kernel name (e.g., Darwin, Linux).
	if f.s {
		v, err := syscall.Sysctl("kern.ostype")
		if err != nil {
			return nil, fmt.Errorf("reading kernel name: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.3: network node hostname.
	if f.n {
		v, err := syscall.Sysctl("kern.hostname")
		if err != nil {
			return nil, fmt.Errorf("reading hostname: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.4: kernel release string.
	if f.r {
		v, err := syscall.Sysctl("kern.osrelease")
		if err != nil {
			return nil, fmt.Errorf("reading kernel release: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.5: kernel version string.
	if f.v {
		v, err := syscall.Sysctl("kern.version")
		if err != nil {
			return nil, fmt.Errorf("reading kernel version: %w", err)
		}
		fields = append(fields, sanitizeVersion(v))
	}
	// R1.6: machine hardware name.
	if f.m {
		v, err := syscall.Sysctl("hw.machine")
		if err != nil {
			return nil, fmt.Errorf("reading machine: %w", err)
		}
		fields = append(fields, v)
	}
	// R1.7: processor type — derived from hw.machine on Darwin.
	// In -a mode, "unknown" is omitted to match GNU uname behavior.
	if f.p {
		pt := processorType()
		if !f.allMode || pt != "unknown" {
			fields = append(fields, pt)
		}
	}
	// R1.8: hardware platform ("unknown" on most platforms).
	// In -a mode, "unknown" is omitted to match GNU uname behavior.
	if f.i {
		hp := "unknown"
		if !f.allMode || hp != "unknown" {
			fields = append(fields, hp)
		}
	}
	// R1.9: operating system name.
	if f.o {
		fields = append(fields, osName())
	}
	return fields, nil
}

// processorType returns the processor type matching guname -p on Darwin.
// GNU coreutils bakes in the processor at configure time; on Darwin
// arm64 maps to "arm" and x86_64 maps to "x86_64".
func processorType() string {
	machine, err := syscall.Sysctl("hw.machine")
	if err != nil {
		return "unknown"
	}
	if machine == "arm64" {
		return "arm"
	}
	return machine
}

// osName returns the operating system name matching guname -o on Darwin.
func osName() string {
	return "Darwin"
}

// sanitizeVersion removes trailing newlines and replaces internal
// newlines with spaces, matching GNU uname -v behavior on Darwin
// where kern.version may contain embedded newlines.
func sanitizeVersion(v string) string {
	v = strings.TrimRight(v, "\n")
	return strings.ReplaceAll(v, "\n", " ")
}
