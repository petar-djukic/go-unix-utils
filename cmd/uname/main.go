// Copyright (c) 2026 Petar Djukic. All rights reserved.
// SPDX-License-Identifier: MIT

// Package main implements cmd/uname: print system information.
// Implements srd044-uname R1.1, R1.2, R1.3, R1.4, R1.5, R1.6, R1.7, R1.8.
package main

import (
	"fmt"
	"os"
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

  -s, --kernel-name        print the kernel name
  -n, --nodename           print the network node hostname
  -r, --kernel-release     print the kernel release
  -v, --kernel-version     print the kernel version
  -m, --machine            print the machine hardware name
  -p, --processor          print the processor type or "unknown"
  -i, --hardware-platform  print the hardware platform or "unknown"
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
func setShortFlags(f *flags, chars string) error {
	for _, c := range chars {
		switch c {
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
		default:
			return fmt.Errorf("invalid option -- '%c'", c)
		}
	}
	return nil
}

// printFields retrieves system info and prints the selected fields
// space-separated on a single line.
// R1.2: -s prints kernel name.
// R1.3: -n prints network node hostname.
// R1.4: -r prints kernel release string.
// R1.5: -v prints kernel version string.
// R1.6: -m prints machine hardware name.
// R1.7: -p prints processor type or "unknown".
// R1.8: -i prints hardware platform or "unknown".
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
		parts = append(parts, processorType(utsname))
	}
	if f.hwPlat {
		parts = append(parts, hardwarePlatform(utsname))
	}

	fmt.Println(strings.Join(parts, " "))
}

// processorType returns the processor type.
// R1.7: On most systems this matches the machine field; returns "unknown"
// if the information is not determinable.
func processorType(uts unix.Utsname) string {
	m := bytesToString(uts.Machine[:])
	if m == "" {
		return "unknown"
	}
	return m
}

// hardwarePlatform returns the hardware platform.
// R1.8: On most systems this matches the machine field; returns "unknown"
// if the information is not determinable.
func hardwarePlatform(uts unix.Utsname) string {
	m := bytesToString(uts.Machine[:])
	if m == "" {
		return "unknown"
	}
	return m
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
