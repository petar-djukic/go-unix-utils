// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/uname: print system information.
// Implements srd044-uname R1.1-R1.9, R2.1, R2.2, R3.1, R3.2.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/petar-djukic/go-unix-utils/pkg/sys"
)

// progName is used in error messages.
const progName = "uname"

// versionText is printed when --version is passed.
const versionText = progName + " (go-unix-utils)"

// helpText is the usage message printed when --help is passed.
const helpText = `Usage: uname [OPTION]...
Print certain system information.  With no OPTION, same as -s.

  -a, --all                print all information, in the following order,
                             except omit -p and -i if unknown:
  -s, --kernel-name        print the kernel name
  -n, --nodename           print the network node hostname
  -r, --kernel-release     print the kernel release
  -v, --kernel-version     print the kernel version
  -m, --machine            print the machine hardware name
  -p, --processor          print the processor type or "unknown"
  -i, --hardware-platform  print the hardware platform or "unknown"
  -o, --operating-system   print the operating system
      --help        display this help and exit
      --version     output version information and exit
`

// flags tracks which information fields the user requested.
type flags struct {
	sysName  bool // -s: kernel name
	nodeName bool // -n: network node hostname
	release  bool // -r: kernel release
	version  bool // -v: kernel version
	machine  bool // -m: machine hardware name
	proc     bool // -p: processor type
	hwPlat   bool // -i: hardware platform
	osName   bool // -o: operating system
	allMode  bool // -a: omit unknown -p and -i
}

func main() {
	sys.InstallSIGPIPEHandler()

	f, err := parseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", progName, err)
		fmt.Fprintf(os.Stderr, "Try '%s --help' for more information.\n", progName)
		os.Exit(1)
	}

	printFields(f)
}

// parseFlags processes command-line arguments and returns the selected flags.
// R1.1: when no flags are given, defaults to -s.
func parseFlags(args []string) (flags, error) {
	var f flags
	anySet := false

	for _, arg := range args {
		if arg == "--help" {
			fmt.Print(helpText)
			os.Exit(0)
		}
		if arg == "--version" {
			fmt.Println(versionText)
			os.Exit(0)
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return f, fmt.Errorf("extra operand '%s'", arg)
		}
		// R3.2: unrecognized long options are reported as such.
		if strings.HasPrefix(arg, "--") {
			return f, fmt.Errorf("unrecognized option '%s'", arg)
		}
		if err := setShortFlags(&f, arg[1:]); err != nil {
			return f, err
		}
		anySet = true
	}

	if !anySet {
		f.sysName = true
	}
	return f, nil
}

// setShortFlags parses a group of short flag characters (e.g., "snrvm").
// R2.1: 'a' enables all fields.
func setShortFlags(f *flags, chars string) error {
	for _, c := range chars {
		switch c {
		case 'a':
			f.allMode = true
			setAllFlags(f)
		case 's':
			f.sysName = true
		case 'n':
			f.nodeName = true
		case 'r':
			f.release = true
		case 'v':
			f.version = true
		case 'm':
			f.machine = true
		case 'p':
			f.proc = true
		case 'i':
			f.hwPlat = true
		case 'o':
			f.osName = true
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// setAllFlags enables all information fields.
// R2.1: -a is equivalent to -snrvmpio.
func setAllFlags(f *flags) {
	f.sysName = true
	f.nodeName = true
	f.release = true
	f.version = true
	f.machine = true
	f.proc = true
	f.hwPlat = true
	f.osName = true
}

// printFields retrieves system info and prints the selected fields
// space-separated on a single line.
// R2.2: fields are printed in canonical order regardless of flag order.
func printFields(f flags) {
	var utsname unix.Utsname
	if err := unix.Uname(&utsname); err != nil {
		fmt.Fprintf(os.Stderr, "%s: cannot get system information: %v\n", progName, err)
		os.Exit(1)
	}

	var parts []string
	if f.sysName {
		parts = append(parts, bytesToString(utsname.Sysname[:]))
	}
	if f.nodeName {
		parts = append(parts, bytesToString(utsname.Nodename[:]))
	}
	if f.release {
		parts = append(parts, bytesToString(utsname.Release[:]))
	}
	if f.version {
		parts = append(parts, bytesToString(utsname.Version[:]))
	}
	if f.machine {
		parts = append(parts, bytesToString(utsname.Machine[:]))
	}
	if f.proc {
		p := processorType(utsname)
		// R2.1: -a omits processor if "unknown".
		if !f.allMode || p != "unknown" {
			parts = append(parts, p)
		}
	}
	if f.hwPlat {
		h := hardwarePlatform(utsname)
		// R2.1: -a omits hardware platform if "unknown".
		if !f.allMode || h != "unknown" {
			parts = append(parts, h)
		}
	}
	if f.osName {
		parts = append(parts, operatingSystem())
	}

	fmt.Println(strings.Join(parts, " "))
}

// processorType returns the processor type.
// R1.7: On Darwin, returns the CPU architecture name matching GNU coreutils
// behavior (e.g., "arm" on ARM64). On Linux, matches the machine field.
// Returns "unknown" if the information is not determinable.
func processorType(uts unix.Utsname) string {
	if runtime.GOOS == "darwin" {
		return darwinProcessorType()
	}
	m := bytesToString(uts.Machine[:])
	if m == "" {
		return "unknown"
	}
	return m
}

// darwinProcessorType returns the processor type on Darwin.
// GNU coreutils on macOS returns the CPU architecture name (e.g., "arm")
// which differs from the machine name (e.g., "arm64").
func darwinProcessorType() string {
	p, err := unix.Sysctl("hw.machine")
	if err != nil || p == "" {
		return "unknown"
	}
	// hw.machine returns "arm64" on Apple Silicon; GNU coreutils
	// reports "arm" to match the base architecture family.
	if base, found := strings.CutSuffix(p, "64"); found {
		return base
	}
	return p
}

// hardwarePlatform returns the hardware platform.
// R1.8: On Darwin, hardware platform is not determinable; returns "unknown"
// matching GNU coreutils behavior. On Linux, matches the machine field.
func hardwarePlatform(uts unix.Utsname) string {
	if runtime.GOOS == "darwin" {
		return "unknown"
	}
	m := bytesToString(uts.Machine[:])
	if m == "" {
		return "unknown"
	}
	return m
}

// operatingSystem returns the operating system name.
// R1.9: On Darwin returns "Darwin"; on Linux returns "GNU/Linux".
func operatingSystem() string {
	switch runtime.GOOS {
	case "darwin":
		return "Darwin"
	case "linux":
		return "GNU/Linux"
	default:
		return runtime.GOOS
	}
}

// bytesToString converts a null-terminated byte array to a Go string.
func bytesToString(raw []byte) string {
	for i, v := range raw {
		if v == 0 {
			return string(raw[:i])
		}
	}
	return string(raw)
}
